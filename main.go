package main

import "os"
import "encoding/json"
import "log"
import "crypto/tls"
import "net"
import "sync"
import "strconv"
import "strings"
import "github.com/emersion/go-sasl"
import "github.com/emersion/go-smtp"
import "io"
import "io/ioutil"
import "time"
import "errors"

// SMTP

type Backend struct {
    cfg TRouteConfig
}
func (b *Backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
    return &Session{ cfg : b.cfg }, nil
}

type Session struct {
    username string
    password string
    from string
    to string
    data string
    cfg TRouteConfig
}
func (s *Session) AuthPlain(username, password string) error {
    s.username = username
    s.password = password
    return nil
}
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
    s.from = from
    return nil
}
func (s *Session) Rcpt(to string) error {
    s.to = to
    return nil
}
func (s *Session) Data(r io.Reader) error {
    if b, err := ioutil.ReadAll(r); err != nil {
        return err
    } else {
        s.data += string(b)
        return nil
    }
}
func (s *Session) Reset() {
    if Config.Is_verbose_log {
        log.Println("smtp: ", s.username, " sends from ", s.from, " to ", s.to, " ", len(s.data), " bytes")
    }
    auth := sasl.NewPlainClient("", s.username, s.password)
    to := []string{s.to}
    err := smtp.SendMail(s.cfg.Destination + ":" + strconv.FormatUint(uint64(s.cfg.Port_out), 10), auth, s.from, to, strings.NewReader(s.data))
    if err != nil {
        log.Println("smtp: ", err)
    }
}
func (s *Session) Logout() error {
    s.username = ""
    s.password = ""
    s.from = ""
    s.to = ""
    s.data = ""
    return nil
}

// TCP and general

type TRouteConfig struct {
    Port_in uint16 `json:"port-in"`
    Port_out uint16 `json:"port-out"`
    Destination string `json:"destination"`
    Type string `json:"type"`
}
type TConfig struct {
    Routes []TRouteConfig `json:"routes"`
    Is_verbose_log bool `json:"verbose-log"`
    SMTP_data_limit int `json:"smtp-data-limit"`
    SMTP_timeout int `json:"smtp-timeout"`
    SMTP_max_recipients int `json:"smtp-max-recipients"`
}
type TState struct {
    is_routing bool
}

var Config = TConfig {
    Is_verbose_log : true,
}

var State = TState {
    is_routing : false,
}

// functions

func load_config() {
    content, err_file := os.ReadFile("./config.json")
    if err_file != nil {
        log.Fatal(err_file)
    }
    err_json := json.Unmarshal(content, &Config)
    if err_json != nil {
        log.Fatal(err_json)
    }
}

func validate_config() {
    /* no validation required at this point */
}

func is_net_closed(err error) bool {
    switch {
        case
            errors.Is(err, net.ErrClosed),
            errors.Is(err, io.EOF):
            return true
        default:
            return false
    }
}

func feedback_tcp(route TRouteConfig, in net.Conn, out net.Conn, wg sync.WaitGroup, is_active *bool) {
    buffer := make([]byte, 1024)
    defer wg.Done()
    for State.is_routing {
        n, err := out.Read(buffer)
        if err != nil {
            if is_net_closed(err) {
                break
            } else {
                log.Println("tcp: ", err)
                continue
            }
        } else {
            if Config.Is_verbose_log {
                log.Println("tcp: received ", n, " bytes")
            }
            if n > 0 {
                _, err := in.Write(buffer[:n])
                if err != nil {
                    if is_net_closed(err) {
                        break
                    } else {
                        log.Println("tcp: ", n, ": ", err)
                        continue
                    }
                } else {
                    log.Println("tcp: relayed successfully")
                }
            }
        }
    }
    log.Print("tcp: closing client")
    *is_active = false
}

func handle_tcp(route TRouteConfig, in net.Conn, wg sync.WaitGroup) {
    defer wg.Done()
    defer in.Close()
    tls_cfg := tls.Config{
        PreferServerCipherSuites: true,
        CipherSuites: []uint16 {
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
            tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
        },
    }
    is_out_active := true
    out, err := tls.Dial("tcp", route.Destination + ":" + strconv.FormatUint(uint64(route.Port_out), 10), &tls_cfg)
    log.Print("tcp: opening client")
    if err != nil {
        log.Println(err)
        return
    }
    defer out.Close()
    wg.Add(1)
    go feedback_tcp(route, in, out, wg, &is_out_active)
    buffer := make([]byte, 1024)
    for State.is_routing && is_out_active {
        n, err := in.Read(buffer)
        if err != nil {
            log.Println("tcp: ", err)
            return
        }
        if n > 0 {
            _, err := out.Write(buffer[:n])
            if err != nil {
                log.Println("tcp: ", n, ": ", err)
                return
            }
        }
    }
}

func listen_route_tcp(route TRouteConfig, wg sync.WaitGroup) {
    defer wg.Done()
    server, err := net.Listen("tcp", ":" + strconv.FormatUint(uint64(route.Port_in), 10))
    if err != nil {
        log.Fatal(err)
    }
    defer server.Close()
    var conwg sync.WaitGroup
    for State.is_routing {
        con, err := server.Accept()
        if err != nil {
            log.Println(err)
            continue
        }
        conwg.Add(1)
        go handle_tcp(route, con, conwg)
    }
    _ = server
    conwg.Wait()
}

func listen_route_smtp(route TRouteConfig, wg sync.WaitGroup) {
    defer wg.Done()
    be := &Backend{ cfg : route }
    s := smtp.NewServer(be)
    s.Addr = ":" + strconv.FormatUint(uint64(route.Port_in), 10)
    s.Domain = "localhost"
    s.ReadTimeout = time.Duration(Config.SMTP_timeout) * time.Second
    s.WriteTimeout = time.Duration(Config.SMTP_timeout) * time.Second
    s.MaxMessageBytes = Config.SMTP_data_limit
    s.MaxRecipients = Config.SMTP_max_recipients
    s.AllowInsecureAuth = true
    if err := s.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}

func listen_route(route TRouteConfig, wg sync.WaitGroup) {
    switch(route.Type) {
    case "tcp":
        listen_route_tcp(route, wg)
    case "smtp":
        listen_route_smtp(route, wg)
    }
}

func run_listeners() {
    var wg sync.WaitGroup
    State.is_routing = true
    if Config.Is_verbose_log {
        log.Printf("starting %d routes", len(Config.Routes))
    }
    for _, route := range Config.Routes {
        wg.Add(1)
        go listen_route(route, wg)
    }
    wg.Wait()
}

func main() {
    load_config()
    validate_config()
    run_listeners()
}
