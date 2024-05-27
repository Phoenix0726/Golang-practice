package codec


import (
    "bufio"
    "encoding/gob"
    "io"
    "log"
)


type GobCodec struct {
    conn io.ReadWriteCloser
    buf *bufio.Writer
    dec *gob.Decoder
    enc *gob.Encoder
}


func NewGobCodec(conn io.ReadWriteCloser) Codec {
    buf := bufio.NewWriter(conn)
    return &GobCodec {
        conn: conn,
        buf: buf,
        dec: gob.NewDecoder(conn),
        enc: gob.NewEncoder(buf),
    }
}


func (c *GobCodec) ReadHeader(header *Header) error {
    return c.dec.Decode(header)
}


func (c *GobCodec) ReadBody(body interface{}) error {
    return c.dec.Decode(body)
}


func (c *GobCodec) Write(header *Header, body interface{}) (err error) {
    defer func() {
        _ = c.buf.Flush()
        if err != nil {
            _ = c.Close()
        }
    } ()

    err = c.enc.Encode(header)
    if err != nil {
        log.Println("rpc codec: gob error encoding header:", err)
        return err
    }

    err = c.enc.Encode(body)
    if err != nil {
        log.Println("rpc codec: gob error encoding body:", err)
        return err
    }

    return nil
}


func (c *GobCodec) Close() error {
    return c.conn.Close()
}
