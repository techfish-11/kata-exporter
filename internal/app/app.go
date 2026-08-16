package app

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/techfish-lab/kata-exporter/internal/config"
	"github.com/techfish-lab/kata-exporter/internal/exporter"
	"github.com/techfish-lab/kata-exporter/internal/switchbot"
)

type Service struct { exp *exporter.Exporter; version string; started time.Time }

func New(cfg config.Config, log *slog.Logger, version string)(*Service,error){
	client:=switchbot.New(cfg.Token,cfg.Secret,cfg.APIBaseURL,cfg.HTTPTimeout)
	return &Service{exp:exporter.New(cfg,client,log,version),version:version,started:time.Now()},nil
}

func (s *Service) Handler() http.Handler {
	mux:=http.NewServeMux()
	mux.HandleFunc("/metrics",func(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return};ctx,cancel:=context.WithTimeout(r.Context(),30*time.Second);defer cancel();b,err:=s.exp.Gather(ctx);if err!=nil{http.Error(w,err.Error(),500);return};w.Header().Set("Content-Type","text/plain; version=0.0.4; charset=utf-8");_,_=w.Write(b)})
	mux.HandleFunc("/-/healthy",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/plain");_,_=io.WriteString(w,"ok\n")})
	mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};w.Header().Set("Content-Type","text/html; charset=utf-8");_ = landing.Execute(w,map[string]string{"Version":s.version})})
	return mux
}

func Check(ctx context.Context,cfg config.Config,w io.Writer)error{
	c:=switchbot.New(cfg.Token,cfg.Secret,cfg.APIBaseURL,cfg.HTTPTimeout);devices,err:=c.Devices(ctx);if err!=nil{return err};fmt.Fprintf(w,"Authentication: OK\nKata Friends devices: %d\n",len(devices));for _,d:=range devices{s,err:=c.DeviceStatus(ctx,d.DeviceID);if err!=nil{fmt.Fprintf(w,"- %s (%s): ERROR: %v\n",d.DeviceName,d.DeviceID,err);continue};fmt.Fprintf(w,"- %s (%s): %s, battery %d%%, mode %s, status %s, firmware %s\n",d.DeviceName,d.DeviceID,s.OnlineStatus,s.Battery,s.Mode,s.Status,s.Version)};if len(devices)==0{return fmt.Errorf("no Kata Friends devices found in this SwitchBot account")};return nil
}

var landing=template.Must(template.New("landing").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Kata Exporter</title><style>body{font:16px system-ui;background:#09111f;color:#e7f2ff;display:grid;place-items:center;min-height:100vh;margin:0}.c{background:#111d31;padding:36px;border:1px solid #25466d;border-radius:20px;box-shadow:0 20px 80px #0008}h1{margin:0 0 8px;color:#70c8ff}a{color:#8fdbff}code{background:#07101e;padding:4px 8px;border-radius:7px}</style></head><body><main class="c"><h1>Kata Exporter</h1><p>SwitchBot Kata Friends → Prometheus</p><p><a href="/metrics">Open metrics</a> · <a href="/-/healthy">Health check</a></p><small>version {{.Version}}</small></main></body></html>`))

