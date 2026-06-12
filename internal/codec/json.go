package codec

import (
    "bytes"
    "encoding/json"
    "sync"
)

var bufferPool = sync.Pool{
    New: func() interface{} {
        return &bytes.Buffer{}
    },
}

func Marshal(v interface{}) ([]byte, error) {
    buf := bufferPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufferPool.Put(buf)
    
    enc := json.NewEncoder(buf)
    if err := enc.Encode(v); err != nil {
        return nil, err
    }
    
    // Make a copy since buf will be reused
    result := make([]byte, buf.Len())
    copy(result, buf.Bytes())
    return result, nil
}

func Unmarshal(data []byte, v interface{}) error {
    buf := bytes.NewReader(data)
    dec := json.NewDecoder(buf)
    return dec.Decode(v)
}

// FastMarshal for small payloads (no copy)
func FastMarshal(v interface{}) ([]byte, error) {
    buf := &bytes.Buffer{}
    enc := json.NewEncoder(buf)
    if err := enc.Encode(v); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}