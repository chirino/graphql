package linkedmap

type LinkedMapEntry struct {
    Value interface{}
    Next  *LinkedMapEntry
}
type LinkedMap struct {
    ValuesByKey map[interface{}]*LinkedMapEntry
    First       *LinkedMapEntry
    Last        *LinkedMapEntry
}

func CreateLinkedMap(size int) *LinkedMap {
    return &LinkedMap{
        ValuesByKey: make(map[interface{}]*LinkedMapEntry, size),
    }
}

func (this *LinkedMap) Get(key interface{}) interface{} {
    entry := this.ValuesByKey[key]
    if entry == nil {
        return nil
    }
    return entry.Value
}

func (this *LinkedMap) Set(key interface{}, value interface{}) interface{} {
    if previousEntry, found := this.ValuesByKey[key]; found {
        prevValue := previousEntry.Value
        entry := this.ValuesByKey[key]
        entry.Value = value
        return prevValue
    }
    entry := &LinkedMapEntry{
        Value: value,
    }
    if this.First == nil {
        this.First = entry
        this.Last = entry
    } else {
        this.Last.Next = entry
        this.Last = entry
    }
    this.ValuesByKey[key] = entry
    return nil
}

