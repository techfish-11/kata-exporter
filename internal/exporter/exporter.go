package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/techfish-lab/kata-exporter/internal/config"
	"github.com/techfish-lab/kata-exporter/internal/metrics"
	"github.com/techfish-lab/kata-exporter/internal/switchbot"
)

type Exporter struct {
	cfg config.Config; client *switchbot.Client; log *slog.Logger; version string
	mu sync.Mutex; cache []byte; cachedAt time.Time
	devices []switchbot.Device; devicesAt time.Time
	diaryMu sync.Mutex; diary map[string]diaryCache
	scrapes uint64; scrapeErrors atomic.Uint64
}
type diaryCache struct { diary switchbot.Diary; at time.Time; err bool }
type result struct { device switchbot.Device; status switchbot.Status; diary switchbot.Diary; statusOK, diaryOK bool }

func New(cfg config.Config, client *switchbot.Client, log *slog.Logger, version string) *Exporter {
	return &Exporter{cfg:cfg,client:client,log:log,version:version,diary:map[string]diaryCache{}}
}

func (e *Exporter) Gather(ctx context.Context) ([]byte,error) {
	e.mu.Lock(); defer e.mu.Unlock()
	if len(e.cache)>0 && time.Since(e.cachedAt)<e.cfg.ScrapeCache { return append([]byte(nil),e.cache...),nil }
	started:=time.Now(); e.scrapes++
	devices, discoverOK := e.resolveDevices(ctx)
	results:=make([]result,len(devices)); sem:=make(chan struct{},e.cfg.MaxConcurrency); var wg sync.WaitGroup
	for i,d:=range devices { wg.Add(1); go func(i int,d switchbot.Device){ defer wg.Done(); sem<-struct{}{}; defer func(){<-sem}(); results[i]=e.collect(ctx,d) }(i,d) }
	wg.Wait(); sort.Slice(results,func(i,j int)bool{return results[i].device.DeviceID<results[j].device.DeviceID})
	w:=&metrics.Writer{}
	e.write(&w,results,discoverOK,time.Since(started))
	e.cache=append(e.cache[:0],w.Bytes()...); e.cachedAt=time.Now()
	return append([]byte(nil),e.cache...),nil
}

func (e *Exporter) resolveDevices(ctx context.Context) ([]switchbot.Device,bool) {
	if len(e.cfg.DeviceIDs)>0 { out:=make([]switchbot.Device,len(e.cfg.DeviceIDs)); for i,id:=range e.cfg.DeviceIDs{out[i]=switchbot.Device{DeviceID:id,DeviceName:id,DeviceType:"Kata Friends"}}; return out,true }
	if len(e.devices)>0 && time.Since(e.devicesAt)<e.cfg.DiscoveryRefresh{return e.devices,true}
	d,err:=e.client.Devices(ctx); if err!=nil {e.log.Error("device discovery failed","error",err);e.scrapeErrors.Add(1);return e.devices,false}
	e.devices=d;e.devicesAt=time.Now();return d,true
}

func (e *Exporter) collect(ctx context.Context,d switchbot.Device) result {
	r:=result{device:d}; s,err:=e.client.DeviceStatus(ctx,d.DeviceID)
	if err!=nil { e.log.Warn("status collection failed","device_id",d.DeviceID,"error",err);e.scrapeErrors.Add(1) } else { r.status=s;r.statusOK=true }
	if !e.cfg.DiaryEnabled{return r}
	e.diaryMu.Lock(); c,ok:=e.diary[d.DeviceID]; e.diaryMu.Unlock()
	if ok && time.Since(c.at)<e.cfg.DiaryRefresh {r.diary=c.diary;r.diaryOK=!c.err;return r}
	end:=time.Now(); diary,err:=e.client.Diary(ctx,d.DeviceID,end.Add(-e.cfg.DiaryWindow),end)
	if err!=nil {e.log.Warn("diary collection failed","device_id",d.DeviceID,"error",err);e.scrapeErrors.Add(1);e.diaryMu.Lock();e.diary[d.DeviceID]=diaryCache{at:end,err:true};e.diaryMu.Unlock();return r}
	r.diary=diary;r.diaryOK=true;e.diaryMu.Lock();e.diary[d.DeviceID]=diaryCache{diary:diary,at:end};e.diaryMu.Unlock();return r
}

func (e *Exporter) write(w *metrics.Writer,rs []result,discoveryOK bool,duration time.Duration) {
	w.Help("kata_exporter_build_info","Build information for Kata Exporter.","gauge");w.Sample("kata_exporter_build_info",map[string]string{"version":e.version},1)
	w.Help("kata_exporter_discovery_success","Whether the latest Kata Friends device discovery succeeded.","gauge");w.Sample("kata_exporter_discovery_success",nil,boolf(discoveryOK))
	w.Help("kata_exporter_scrapes_total","Total exporter collections.","counter");w.Sample("kata_exporter_scrapes_total",nil,float64(e.scrapes))
	w.Help("kata_exporter_scrape_errors_total","Total device collection errors.","counter");w.Sample("kata_exporter_scrape_errors_total",nil,float64(e.scrapeErrors.Load()))
	w.Help("kata_exporter_scrape_duration_seconds","Duration of the latest exporter collection.","gauge");w.Sample("kata_exporter_scrape_duration_seconds",nil,duration.Seconds())
	reqs,errs:=e.client.Counts();w.Help("kata_exporter_api_requests_total","Total SwitchBot API requests.","counter");w.Sample("kata_exporter_api_requests_total",nil,float64(reqs));w.Help("kata_exporter_api_errors_total","Total failed SwitchBot API requests.","counter");w.Sample("kata_exporter_api_errors_total",nil,float64(errs))
	declareDeviceMetrics(w)
	for _,r:=range rs { e.writeDevice(w,r) }
}

func declareDeviceMetrics(w *metrics.Writer) {
	for _,d:=range [][3]string{{"kata_up","Whether status collection succeeded for the device.","gauge"},{"kata_info","Static Kata device information.","gauge"},{"kata_battery_percent","Current device battery percentage.","gauge"},{"kata_online","Whether the device reports online.","gauge"},{"kata_mode_info","Current mode as an info-style metric.","gauge"},{"kata_status_info","Current activity status as an info-style metric.","gauge"},{"kata_child_lock","Whether child lock is enabled.","gauge"},{"kata_hospitalized","Whether the device is in a service state.","gauge"},{"kata_diary_collection_success","Whether the latest diary collection succeeded.","gauge"},{"kata_diary_events","Event diary records in the configured rolling window.","gauge"},{"kata_ai_diaries","AI text diary records in the configured rolling window.","gauge"},{"kata_comic_diaries","AI comic diary records in the configured rolling window.","gauge"},{"kata_diary_last_event_timestamp_seconds","Timestamp of the newest diary record.","gauge"}} {w.Help(d[0],d[1],d[2])}
}

func (e *Exporter) writeDevice(w *metrics.Writer,r result) {
	base:=map[string]string{"device_id":r.device.DeviceID,"device_name":r.device.DeviceName}
	w.Sample("kata_up",base,boolf(r.statusOK)); if !r.statusOK{return}
	w.Sample("kata_info",merge(base,map[string]string{"firmware":r.status.Version,"device_type":r.status.DeviceType}),1)
	w.Sample("kata_battery_percent",base,float64(r.status.Battery));w.Sample("kata_online",base,boolf(r.status.OnlineStatus=="online"))
	w.Sample("kata_mode_info",merge(base,map[string]string{"mode":r.status.Mode}),1);w.Sample("kata_status_info",merge(base,map[string]string{"status":r.status.Status}),1)
	w.Sample("kata_child_lock",base,boolf(r.status.ChildLock=="on")); state:=hospitalState(r.status.Hospitalized);w.Sample("kata_hospitalized",merge(base,map[string]string{"state":state}),boolf(r.status.Hospitalized!=0))
	if e.cfg.DiaryEnabled {w.Sample("kata_diary_collection_success",base,boolf(r.diaryOK));if r.diaryOK{window:=e.cfg.DiaryWindow.String();labels:=merge(base,map[string]string{"window":window});w.Sample("kata_diary_events",labels,float64(len(r.diary.Diary)));w.Sample("kata_ai_diaries",labels,float64(len(r.diary.DiaryAI)));w.Sample("kata_comic_diaries",labels,float64(len(r.diary.ComicDiaryAI)));w.Sample("kata_diary_last_event_timestamp_seconds",base,float64(latest(r.diary))/1000)}}
}

func latest(d switchbot.Diary) int64 {var x int64;for _,v:=range d.Diary{if v.Timestamp>x{x=v.Timestamp}};for _,v:=range d.DiaryAI{if v.Timestamp>x{x=v.Timestamp}};for _,v:=range d.ComicDiaryAI{if v.Timestamp>x{x=v.Timestamp}};return x}
func boolf(v bool)float64{if v{return 1};return 0}
func merge(a,b map[string]string)map[string]string{o:=make(map[string]string,len(a)+len(b));for k,v:=range a{o[k]=v};for k,v:=range b{o[k]=v};return o}
func hospitalState(n int)string{switch n{case 0:return "normal";case 1:return "repair";case 2:return "maintenance";case 3:return "cleaning";default:return fmt.Sprintf("unknown_%d",n)}}
