package algorithm

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"time"
)

// http://daizuozhuo.github.io/golang-rpc-practice/

func TimeoutCoder(f func(interface{}) error, e interface{}, msg string) error {
	echan := make(chan error, 1)
	go func() { echan <- f(e) }()
	select {
	case e := <-echan:
		return e
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timeout %s", msg)
	}
}

type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

func (c *gobServerCodec) ReadRequestHeader(r *rpc.Request) error {
	return TimeoutCoder(c.dec.Decode, r, "server read request header")
}

func (c *gobServerCodec) ReadRequestBody(body interface{}) error {
	return TimeoutCoder(c.dec.Decode, body, "server read request body")
}

func (c *gobServerCodec) WriteResponse(r *rpc.Response, body interface{}) (err error) {
	if err = TimeoutCoder(c.enc.Encode, r, "server write response"); err != nil {
		if c.encBuf.Flush() == nil {
			fmt.Printf("rpc: gob error encoding response: %s\n", err)
			c.Close()
		}
		return
	}
	if err = TimeoutCoder(c.enc.Encode, body, "server write response body"); err != nil {
		if c.encBuf.Flush() == nil {
			fmt.Printf("rpc: gob error encoding body: %s\n", err)
			c.Close()
		}
		return
	}
	return c.encBuf.Flush()
}

func (c *gobServerCodec) Close() error {
	if c.closed {
		// Only call c.rwc.Close once; otherwise the semantics are undefined.
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

func ListenRPC(portAddr string, worker interface{}, exitSignal chan bool) string {
	handler := rpc.NewServer()
	handler.Register(worker)
	l, e := net.Listen("tcp", portAddr)
	if e != nil {
		log.Fatal("Error: listen error:", e)
	}
	go func() {
		defer l.Close()
		defer func() {
			r := recover()
			if r != nil{
				fmt.Printf("%s\n", r)
			}
		}()
		for {
			select{
			case <-exitSignal:
				exitSignal <- true
				return
			default:
			}

			conn, err := l.Accept()
			if err != nil {
				fmt.Printf("Error: accept rpc connection %s", err.Error())
				continue
			}
			defer conn.Close()
			go func(conn net.Conn) {
				defer func() {
					r := recover()
					if r != nil{
						fmt.Printf("%s\n", r)
					}
				}()
				buf := bufio.NewWriter(conn)
				srv := &gobServerCodec{
					rwc:    conn,
					dec:    gob.NewDecoder(conn),
					enc:    gob.NewEncoder(buf),
					encBuf: buf,
				}
				err = handler.ServeRequest(srv)
				if err != nil {
					if(err.Error() != "EOF"){
						fmt.Printf("Error: server rpc request %s", err.Error())
					}
				}
				srv.Close()
			}(conn)
		}
	}()
	return l.Addr().String()
}

type gobClientCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func (c *gobClientCodec) WriteRequest(r *rpc.Request, body interface{}) (err error) {
	if err = TimeoutCoder(c.enc.Encode, r, "client write request"); err != nil {
		return
	}
	if err = TimeoutCoder(c.enc.Encode, body, "client write request body"); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobClientCodec) ReadResponseHeader(r *rpc.Response) error {
	return c.dec.Decode(r)
}

func (c *gobClientCodec) ReadResponseBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobClientCodec) Close() error {
	return c.rwc.Close()
}

func RpcCall(srv string, rpcname string, args interface{}, reply interface{}, timeout time.Duration) (ret error) {
	defer func() {
		if r := recover(); r != nil {
			// fail gracefully
			ret = fmt.Errorf("%s", r)
		}
	}()

	conn, err := net.DialTimeout("tcp", srv, timeout)
	defer conn.Close()
	if err != nil {
		return fmt.Errorf("ConnectError: %s", err.Error())
	}
	encBuf := bufio.NewWriter(conn)
	codec := &gobClientCodec{conn, gob.NewDecoder(conn), gob.NewEncoder(encBuf), encBuf}
	c := rpc.NewClientWithCodec(codec)
	err = c.Call(rpcname, args, reply)
	errc := c.Close()
	if err != nil && errc != nil {
		return fmt.Errorf("%s %s", err, errc)
	}
	if err != nil {
		return err
	} else {
		return errc
	}
}