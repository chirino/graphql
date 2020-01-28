//go:generate go run .

// This whole directory is used to generate the assets_vfsdata.go file.  It's not compiled into the binary.
//
package main

import (
    "github.com/chirino/graphql/internal/filesystem"
    "github.com/shurcooL/httpfs/filter"
    "github.com/shurcooL/vfsgen"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

func main() {
    err := vfsgen.Generate(GetAssetsFS(), vfsgen.Options{
        Filename:        filepath.Join("..", "assets", "assets.go"),
        PackageName:     "assets",
        BuildTags:       "",
        VariableName:    "FileSystem",
        VariableComment: "",
    })
    if err != nil {
        log.Fatalln(err)
    }
}

func GetAssetsFS() http.FileSystem {
    assetsDir := filepath.Join("..", "assets")
    return filesystem.NewFileInfoMappingFS(filter.Keep(http.Dir(assetsDir), func(path string, fi os.FileInfo) bool {
        if fi.Name() == ".DS_Store" {
            return false
        }
        if strings.HasSuffix(fi.Name(), ".go") {
            return false
        }
        return true
    }), func(fi os.FileInfo) (os.FileInfo, error) {
        return &zeroTimeFileInfo{fi}, nil
    })
}

type zeroTimeFileInfo struct {
    os.FileInfo
}

func (*zeroTimeFileInfo) ModTime() time.Time {
    return time.Time{}
}
