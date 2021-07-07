// This file is a modified version of the text/scanner package

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package scanner provides a scanner and tokenizer for UTF-8-encoded text.
// It takes an io.Reader providing the source, which then can be tokenized
// through repeated calls to the Scan function. For compatibility with
// existing tools, the NUL character is not allowed. If the first character
// in the source is a UTF-8 encoded byte order mark (BOM), it is discarded.
//
// By default, a Scanner skips white space and comments and recognizes all
// literals as defined by the GraphQL specification.
package scanner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"text/scanner"
	"unicode"
	"unicode/utf8"

	"github.com/josharian/intern"
)

// A source position is represented by a Position value.
// A position is valid if Line > 0.
type Position struct {
	Filename string // filename, if any
	Offset   int    // byte offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count per line)
}

// IsValid reports whether the position is valid.
func (pos *Position) IsValid() bool { return pos.Line > 0 }

func (pos Position) String() string {
	s := pos.Filename
	if s == "" {
		s = "<input>"
	}
	if pos.IsValid() {
		s += fmt.Sprintf(":%d:%d", pos.Line, pos.Column)
	}
	return s
}

// The result of Scan is one of these tokens or a Unicode character.
const (
	EOF = -(iota + 1)
	Ident
	Int
	Float
	String
	BlockString
	Comment

	// internal use only
	skipComment
)

var tokenString = map[rune]string{
	EOF:         "EOF",
	Ident:       "Ident",
	Int:         "Int",
	Float:       "Float",
	String:      "String",
	BlockString: "BlockString",
	Comment:     "Comment",
}

// TokenString returns a printable string for a token or Unicode character.
func TokenString(tok rune) string {
	if s, found := tokenString[tok]; found {
		return s
	}
	return fmt.Sprintf("%q", string(tok))
}

const Whitespace uint64 = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '

const bufLen = 1024 // at least utf8.UTFMax

// A Scanner implements reading of Unicode characters and tokens from an io.Reader.
type Scanner struct {
	// Input
	src io.Reader

	// Source buffer
	srcBuf [bufLen + 1]byte // +1 for sentinel for common case of s.next()
	srcPos int              // reading position (srcBuf index)
	srcEnd int              // source end (srcBuf index)

	// Source position
	srcBufOffset int // byte offset of srcBuf[0] in source
	line         int // line count
	column       int // character count
	lastLineLen  int // length of last line in characters (for correct column reporting)
	lastCharLen  int // length of last character in bytes

	// Token text buffer
	// Typically, token text is stored completely in srcBuf, but in general
	// the token text's head may be buffered in tokBuf while the token text's
	// tail is stored in srcBuf.
	TokBuf bytes.Buffer // token text head that is not in srcBuf anymore
	tokPos int          // token text tail position (srcBuf index); valid if >= 0
	tokEnd int          // token text tail end (srcBuf index)

	// One character look-ahead
	ch rune // character before current srcPos

	// Error is called for each error encountered. If no Error
	// function is set, the error is reported to os.Stderr.
	Error func(s *Scanner, msg string)

	// ErrorCount is incremented by one for each error encountered.
	ErrorCount int

	// IsIdentRune is a predicate controlling the characters accepted
	// as the ith rune in an identifier. The set of valid characters
	// must not intersect with the set of white space characters.
	// If no IsIdentRune function is set, regular Go identifiers are
	// accepted instead. The field may be changed at any time.
	IsIdentRune func(ch rune, i int) bool

	// Start position of most recently scanned token; set by Scan.
	// Calling Init or Next invalidates the position (Line == 0).
	// The Filename field is always left untouched by the Scanner.
	// If an error is reported (via Error) and Position is invalid,
	// the scanner is not inside a token. Call Pos to obtain an error
	// position in that case, or to obtain the position immediately
	// after the most recently scanned token.
	Position
}

// Init initializes a Scanner with a new source and returns s.
// Error is set to nil, ErrorCount is set to 0, Mode is set to GoTokens,
// and Whitespace is set to GoWhitespace.
func (s *Scanner) Init(src io.Reader) *Scanner {
	s.src = src

	// initialize source buffer
	// (the first call to next() will fill it by calling src.Read)
	s.srcBuf[0] = utf8.RuneSelf // sentinel
	s.srcPos = 0
	s.srcEnd = 0

	// initialize source position
	s.srcBufOffset = 0
	s.line = 1
	s.column = 0
	s.lastLineLen = 0
	s.lastCharLen = 0

	// initialize token text buffer
	// (required for first call to next()).
	s.tokPos = -1

	// initialize one character look-ahead
	s.ch = -2 // no char read yet, not EOF

	// initialize public fields
	s.Error = nil
	s.ErrorCount = 0
	s.Line = 0 // invalidate token position

	return s
}

// next reads and returns the next Unicode character. It is designed such
// that only a minimal amount of work needs to be done in the common ASCII
// case (one test to check for both ASCII and end-of-buffer, and one test
// to check for newlines).
func (s *Scanner) next() rune {
	ch, width := rune(s.srcBuf[s.srcPos]), 1

	if ch >= utf8.RuneSelf {
		// uncommon case: not ASCII or not enough bytes
		for s.srcPos+utf8.UTFMax > s.srcEnd && !utf8.FullRune(s.srcBuf[s.srcPos:s.srcEnd]) {
			// not enough bytes: read some more, but first
			// save away token text if any
			if s.tokPos >= 0 {
				s.TokBuf.Write(s.srcBuf[s.tokPos:s.srcPos])
				s.tokPos = 0
				// s.tokEnd is set by Scan()
			}
			// move unread bytes to beginning of buffer
			copy(s.srcBuf[0:], s.srcBuf[s.srcPos:s.srcEnd])
			s.srcBufOffset += s.srcPos
			// read more bytes
			// (an io.Reader must return io.EOF when it reaches
			// the end of what it is reading - simply returning
			// n == 0 will make this loop retry forever; but the
			// error is in the reader implementation in that case)
			i := s.srcEnd - s.srcPos
			n, err := s.src.Read(s.srcBuf[i:bufLen])
			s.srcPos = 0
			s.srcEnd = i + n
			s.srcBuf[s.srcEnd] = utf8.RuneSelf // sentinel
			if err != nil {
				if err != io.EOF {
					s.error(err.Error())
				}
				if s.srcEnd == 0 {
					if s.lastCharLen > 0 {
						// previous character was not EOF
						s.column++
					}
					s.lastCharLen = 0
					return EOF
				}
				// If err == EOF, we won't be getting more
				// bytes; break to avoid infinite loop. If
				// err is something else, we don't know if
				// we can get more bytes; thus also break.
				break
			}
		}
		// at least one byte
		ch = rune(s.srcBuf[s.srcPos])
		if ch >= utf8.RuneSelf {
			// uncommon case: not ASCII
			ch, width = utf8.DecodeRune(s.srcBuf[s.srcPos:s.srcEnd])
			if ch == utf8.RuneError && width == 1 {
				// advance for correct error position
				s.srcPos += width
				s.lastCharLen = width
				s.column++
				s.error("invalid UTF-8 encoding")
				return ch
			}
		}
	}

	// advance
	s.srcPos += width
	s.lastCharLen = width
	s.column++

	// special situations
	switch ch {
	case 0:
		// for compatibility with other tools
		s.error("invalid character NUL")
	case '\n':
		s.line++
		s.lastLineLen = s.column
		s.column = 0
	}

	return ch
}

// Next reads and returns the next Unicode character.
// It returns EOF at the end of the source. It reports
// a read error by calling s.Error, if not nil; otherwise
// it prints an error message to os.Stderr. Next does not
// update the Scanner's Position field; use Pos() to
// get the current position.
func (s *Scanner) Next() rune {
	s.tokPos = -1 // don't collect token text
	s.Line = 0    // invalidate token position
	ch := s.Peek()
	if ch != EOF {
		s.ch = s.next()
	}
	return ch
}

// Peek returns the next Unicode character in the source without advancing
// the scanner. It returns EOF if the scanner's position is at the last
// character of the source.
func (s *Scanner) Peek() rune {
	if s.ch == -2 {
		// this code is only run for the very first character
		s.ch = s.next()
		if s.ch == '\uFEFF' {
			s.ch = s.next() // ignore BOM
		}
	}
	return s.ch
}

func (s *Scanner) error(msg string) {
	s.tokEnd = s.srcPos - s.lastCharLen // make sure token text is terminated
	s.ErrorCount++
	if s.Error != nil {
		s.Error(s, msg)
		return
	}
	pos := s.Position
	if !pos.IsValid() {
		pos = s.Pos()
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", pos, msg)
}

func (s *Scanner) errorf(format string, args ...interface{}) {
	s.error(fmt.Sprintf(format, args...))
}

func (s *Scanner) isIdentRune(ch rune, i int) bool {
	if s.IsIdentRune != nil {
		return s.IsIdentRune(ch, i)
	}
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
}

func (s *Scanner) scanIdentifier() rune {
	// we know the zero'th rune is OK; start scanning at the next one
	ch := s.next()
	for i := 1; s.isIdentRune(ch, i); i++ {
		ch = s.next()
	}
	return ch
}

func lower(ch rune) rune     { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }
func isHex(ch rune) bool     { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }

// digits accepts the sequence { digit | '_' } starting with ch0.
// If base <= 10, digits accepts any decimal digit but records
// the first invalid digit >= base in *invalid if *invalid == 0.
// digits returns the first rune that is not part of the sequence
// anymore, and a bitset describing whether the sequence contained
// digits (bit 0 is set), or separators '_' (bit 1 is set).
func (s *Scanner) digits(ch0 rune, base int, invalid *rune) (ch rune, digsep int) {
	ch = ch0
	if base <= 10 {
		max := rune('0' + base)
		for isDecimal(ch) || ch == '_' {
			ds := 1
			if ch == '_' {
				ds = 2
			} else if ch >= max && *invalid == 0 {
				*invalid = ch
			}
			digsep |= ds
			ch = s.next()
		}
	} else {
		for isHex(ch) || ch == '_' {
			ds := 1
			if ch == '_' {
				ds = 2
			}
			digsep |= ds
			ch = s.next()
		}
	}
	return
}

func (s *Scanner) scanNumber(ch rune, seenDot bool) (rune, rune) {
	base := 10         // number base
	prefix := rune(0)  // one of 0 (decimal), '0' (0-octal), 'x', 'o', or 'b'
	digsep := 0        // bit 0: digit present, bit 1: '_' present
	invalid := rune(0) // invalid digit in literal, or 0

	// integer part
	var tok rune
	var ds int
	if !seenDot {
		tok = Int
		if ch == '0' {
			ch = s.next()
			switch lower(ch) {
			case 'x':
				ch = s.next()
				base, prefix = 16, 'x'
			case 'o':
				ch = s.next()
				base, prefix = 8, 'o'
			case 'b':
				ch = s.next()
				base, prefix = 2, 'b'
			default:
				base, prefix = 8, '0'
				digsep = 1 // leading 0
			}
		}
		ch, ds = s.digits(ch, base, &invalid)
		digsep |= ds
		if ch == '.' {
			ch = s.next()
			seenDot = true
		}
	}

	// fractional part
	if seenDot {
		tok = Float
		if prefix == 'o' || prefix == 'b' {
			s.error("invalid radix point in " + litname(prefix))
		}
		ch, ds = s.digits(ch, base, &invalid)
		digsep |= ds
	}

	if digsep&1 == 0 {
		s.error(litname(prefix) + " has no digits")
	}

	// exponent
	if e := lower(ch); e == 'e' || e == 'p' {
		switch {
		case e == 'e' && prefix != 0 && prefix != '0':
			s.errorf("%q exponent requires decimal mantissa", ch)
		case e == 'p' && prefix != 'x':
			s.errorf("%q exponent requires hexadecimal mantissa", ch)
		}
		ch = s.next()
		tok = Float
		if ch == '+' || ch == '-' {
			ch = s.next()
		}
		ch, ds = s.digits(ch, 10, nil)
		digsep |= ds
		if ds&1 == 0 {
			s.error("exponent has no digits")
		}
	} else if prefix == 'x' && tok == Float {
		s.error("hexadecimal mantissa requires a 'p' exponent")
	}

	if tok == Int && invalid != 0 {
		s.errorf("invalid digit %q in %s", invalid, litname(prefix))
	}

	if digsep&2 != 0 {
		s.tokEnd = s.srcPos - s.lastCharLen // make sure token text is terminated
		if i := invalidSep(s.TokenBytes()); i >= 0 {
			s.error("'_' must separate successive digits")
		}
	}

	return tok, ch
}

func litname(prefix rune) string {
	switch prefix {
	default:
		return "decimal literal"
	case 'x':
		return "hexadecimal literal"
	case 'o', '0':
		return "octal literal"
	case 'b':
		return "binary literal"
	}
}

// invalidSep returns the index of the first invalid separator in x, or -1.
func invalidSep(x []byte) int {
	x1 := ' ' // prefix char, we only care if it's 'x'
	d := '.'  // digit, one of '_', '0' (a digit), or '.' (anything else)
	i := 0

	// a prefix counts as a digit
	if len(x) >= 2 && x[0] == '0' {
		x1 = lower(rune(x[1]))
		if x1 == 'x' || x1 == 'o' || x1 == 'b' {
			d = '0'
			i = 2
		}
	}

	// mantissa and exponent
	for ; i < len(x); i++ {
		p := d // previous digit
		d = rune(x[i])
		switch {
		case d == '_':
			if p != '0' {
				return i
			}
		case isDecimal(d) || x1 == 'x' && isHex(d):
			d = '0'
		default:
			if p == '_' {
				return i - 1
			}
			d = '.'
		}
	}
	if d == '_' {
		return len(x) - 1
	}

	return -1
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= lower(ch) && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanDigits(ch rune, base, n int) rune {
	for n > 0 && digitVal(ch) < base {
		ch = s.next()
		n--
	}
	if n > 0 {
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanEscape(quote rune) rune {
	ch := s.next() // read character after '/'
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		// nothing to do
		ch = s.next()
	case '0', '1', '2', '3', '4', '5', '6', '7':
		ch = s.scanDigits(ch, 8, 3)
	case 'x':
		ch = s.scanDigits(s.next(), 16, 2)
	case 'u':
		ch = s.scanDigits(s.next(), 16, 4)
	case 'U':
		ch = s.scanDigits(s.next(), 16, 8)
	default:
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanString() (n int) {
	ch := s.next() // read character after quote
	for ch != '"' {
		if ch == '\n' || ch < 0 {
			s.error("literal not terminated")
			return
		}
		if ch == '\\' {
			ch = s.scanEscape('"')
		} else {
			ch = s.next()
		}
		n++
	}
	return
}

func (s *Scanner) scanBlockString() (n int) {
	quoteCounter := 0
	for {
		ch := s.next() // read character after quote
		if ch == scanner.EOF {
			s.error("string block literal not terminated")
			return
		}
		if ch == '"' {
			quoteCounter += 1
			if quoteCounter == 3 {
				return BlockString
			}
		} else {
			quoteCounter = 0
		}
	}
}

func (s *Scanner) scanComment() rune {
	// line comment
	ch := s.next()
	for ch != '\n' && ch >= 0 {
		ch = s.next()
	}
	return ch
}

// Scan reads the next token or Unicode character from source and returns it.
// It only recognizes tokens t for which the respective Mode bit (1<<-t) is set.
// It returns EOF at the end of the source. It reports scanner errors (read and
// token errors) by calling s.Error, if not nil; otherwise it prints an error
// message to os.Stderr.
func (s *Scanner) Scan() rune {
	ch := s.Peek()

	// reset token text position
	s.tokPos = -1
	s.Line = 0

redo:
	// skip white space
	for Whitespace&(1<<uint(ch)) != 0 {
		ch = s.next()
	}

	// start collecting token text
	s.TokBuf.Reset()
	s.tokPos = s.srcPos - s.lastCharLen

	// set token position
	// (this is a slightly optimized version of the code in Pos())
	s.Offset = s.srcBufOffset + s.tokPos
	if s.column > 0 {
		// common case: last character was not a '\n'
		s.Line = s.line
		s.Column = s.column
	} else {
		// last character was a '\n'
		// (we cannot be at the beginning of the source
		// since we have called next() at least once)
		s.Line = s.line - 1
		s.Column = s.lastLineLen
	}

	// determine token value
	tok := ch
	switch {
	case s.isIdentRune(ch, 0):
		tok = Ident
		ch = s.scanIdentifier()
	case isDecimal(ch):
		tok, ch = s.scanNumber(ch, false)
	default:
		switch ch {
		case EOF:
			break
		case '"':
			ch2 := s.next()
			if ch2 == '"' {
				ch3 := s.next()
				if ch3 == '"' {
					s.scanBlockString()
					tok = BlockString
					ch = s.next()
					break
				} else {
					tok = String
					// It was just an empty String...
					break
				}
			}
			s.scanString()
			tok = String
			ch = s.next()
			break
		case '.':
			ch = s.next()
			if isDecimal(ch) {
				tok, ch = s.scanNumber(ch, true)
			}
		case '#':
			s.tokPos = -1 // don't collect token text
			ch = s.scanComment()
			goto redo
		default:
			ch = s.next()
		}
	}

	// end of token text
	s.tokEnd = s.srcPos - s.lastCharLen

	s.ch = ch
	return tok
}

// Pos returns the position of the character immediately after
// the character or token returned by the last call to Next or Scan.
// Use the Scanner's Position field for the start position of the most
// recently scanned token.
func (s *Scanner) Pos() (pos Position) {
	pos.Filename = s.Filename
	pos.Offset = s.srcBufOffset + s.srcPos - s.lastCharLen
	switch {
	case s.column > 0:
		// common case: last character was not a '\n'
		pos.Line = s.line
		pos.Column = s.column
	case s.lastLineLen > 0:
		// last character was a '\n'
		pos.Line = s.line - 1
		pos.Column = s.lastLineLen
	default:
		// at the beginning of the source
		pos.Line = 1
		pos.Column = 1
	}
	return
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling Scan and in calls of Scanner.Error.
func (s *Scanner) TokenText() string {
	if s.tokPos < 0 {
		// no token text
		return ""
	}

	if s.tokEnd < s.tokPos {
		// if EOF was reached, s.tokEnd is set to -1 (s.srcPos == 0)
		s.tokEnd = s.tokPos
	}
	// s.tokEnd >= s.tokPos

	if s.TokBuf.Len() == 0 {
		// common case: the entire token text is still in srcBuf
		return string(s.srcBuf[s.tokPos:s.tokEnd])
	}

	// part of the token text was saved in tokBuf: save the rest in
	// tokBuf as well and return its content
	s.TokBuf.Write(s.srcBuf[s.tokPos:s.tokEnd])
	s.tokPos = s.tokEnd // ensure idempotency of TokenText() call
	return s.TokBuf.String()
}

func (s *Scanner) TextAt(offset int, len int) string {
	offset = offset - s.srcBufOffset
	if offset < 0 {
		return ""
	}
	return string(s.srcBuf[offset : offset+len])
}

// TokenText returns the interned string corresponding to the most recently scanned token.
// Valid after calling Scan and in calls of Scanner.Error.
func (s *Scanner) TokenTextIntern() string {
	if s.tokPos < 0 {
		// no token text
		return ""
	}

	if s.tokEnd < s.tokPos {
		// if EOF was reached, s.tokEnd is set to -1 (s.srcPos == 0)
		s.tokEnd = s.tokPos
	}
	// s.tokEnd >= s.tokPos

	if s.TokBuf.Len() == 0 {
		// common case: the entire token text is still in srcBuf
		return intern.Bytes(s.srcBuf[s.tokPos:s.tokEnd])
	}

	// part of the token text was saved in tokBuf: save the rest in
	// tokBuf as well and return its content
	s.TokBuf.Write(s.srcBuf[s.tokPos:s.tokEnd])
	s.tokPos = s.tokEnd // ensure idempotency of TokenText() call
	return intern.Bytes(s.TokBuf.Bytes())
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling Scan and in calls of Scanner.Error.
func (s *Scanner) TokenBytes() []byte {
	if s.tokPos < 0 {
		// no token text
		return []byte{}
	}

	if s.tokEnd < s.tokPos {
		// if EOF was reached, s.tokEnd is set to -1 (s.srcPos == 0)
		s.tokEnd = s.tokPos
	}
	// s.tokEnd >= s.tokPos

	if s.TokBuf.Len() == 0 {
		// common case: the entire token text is still in srcBuf
		return s.srcBuf[s.tokPos:s.tokEnd]
	}

	// part of the token text was saved in tokBuf: save the rest in
	// tokBuf as well and return its content
	s.TokBuf.Write(s.srcBuf[s.tokPos:s.tokEnd])
	s.tokPos = s.tokEnd // ensure idempotency of TokenText() call
	return s.TokBuf.Bytes()
}
