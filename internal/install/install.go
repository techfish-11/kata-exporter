package install

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/techfish-lab/kata-exporter/internal/config"
)

const managedBegin = "# KATA_EXPORTER_MANAGED_BEGIN"
const managedEnd = "# KATA_EXPORTER_MANAGED_END"

func Run(args []string,version string)error{
	fs:=flag.NewFlagSet("install",flag.ContinueOnError); token:=fs.String("token","","SwitchBot Open Token (prefer KATA_TOKEN)");secret:=fs.String("secret","","SwitchBot secret (prefer KATA_SECRET)");devices:=fs.String("device-ids","","comma-separated device IDs; empty enables auto-discovery");listen:=fs.String("listen",":9788","exporter listen address");prom:=fs.String("prometheus-config",detect([]string{"/etc/prometheus/prometheus.yml","/etc/prometheus/prometheus.yaml"}),"Prometheus config to patch; empty skips");grafana:=fs.String("grafana-dir",detectDir([]string{"/var/lib/grafana/dashboards","/usr/share/grafana/public/dashboards"}),"dashboard directory; empty skips");noStart:=fs.Bool("no-start",false,"write files but do not start/reload services");nonInteractive:=fs.Bool("non-interactive",false,"fail instead of prompting for credentials");if err:=fs.Parse(args);err!=nil{return err}
	if runtime.GOOS!="linux"{return errors.New("automatic install currently supports Linux/systemd; use 'serve' on other platforms")};if os.Geteuid()!=0{return errors.New("install must run as root (sudo -E kata-exporter install)")}
	if *token==""{*token=os.Getenv("KATA_TOKEN")};if *secret==""{*secret=os.Getenv("KATA_SECRET")}
	if !*nonInteractive{r:=bufio.NewReader(os.Stdin);if *token==""{*token=prompt(r,"SwitchBot Open Token")};if *secret==""{*secret=prompt(r,"SwitchBot Secret")}}
	if *token==""||*secret==""{return errors.New("token and secret are required; set KATA_TOKEN/KATA_SECRET or pass flags")}
	cfg:=config.Default();cfg.Token=*token;cfg.Secret=*secret;cfg.Listen=*listen;if *devices!=""{for _,d:=range strings.Split(*devices,","){if d=strings.TrimSpace(d);d!=""{cfg.DeviceIDs=append(cfg.DeviceIDs,d)}}};b,err:=cfg.Marshal();if err!=nil{return err}
	exe,err:=os.Executable();if err!=nil{return err};exe,err=filepath.EvalSymlinks(exe);if err!=nil{return err}
	steps:=[]struct{path string;data []byte;mode os.FileMode}{{"/usr/local/bin/kata-exporter",nil,0755},{"/etc/kata-exporter/config.json",b,0600},{"/etc/systemd/system/kata-exporter.service",[]byte(systemdUnit),0644}}
	for _,s:=range steps{if s.data==nil{if err:=copyFile(exe,s.path,s.mode);err!=nil{return err}}else if err:=writeAtomic(s.path,s.data,s.mode);err!=nil{return err}}
	if err:=ensureServiceUser();err!=nil{return err}
	if *prom!=""{if err:=patchPrometheus(*prom);err!=nil{return err}}
	if *grafana!=""{if err:=writeAtomic(filepath.Join(*grafana,"kata-exporter.json"),[]byte(dashboard),0644);err!=nil{return err};provider:="/etc/grafana/provisioning/dashboards/kata-exporter.yml";data:=[]byte(fmt.Sprintf(grafanaProvider,*grafana));if err:=writeAtomic(provider,data,0644);err!=nil{return err}}
	if !*noStart { _=run("systemctl","daemon-reload");if err:=run("systemctl","enable","--now","kata-exporter.service");err!=nil{return err};if *prom!=""{_ = run("systemctl","reload","prometheus.service")};if *grafana!=""{_ = run("systemctl","restart","grafana-server.service")}}
	fmt.Printf("Kata Exporter %s installed.\nMetrics: http://127.0.0.1:9788/metrics\nConfig: /etc/kata-exporter/config.json\n",version)
	if *prom==""{fmt.Println("Prometheus auto-configuration was skipped. Add the output of 'kata-exporter print-dashboard' to Grafana and scrape localhost:9788 manually.")}
	return nil
}

func Uninstall(args []string)error{fs:=flag.NewFlagSet("uninstall",flag.ContinueOnError);purge:=fs.Bool("purge",false,"also remove config and dashboards");prom:=fs.String("prometheus-config",detect([]string{"/etc/prometheus/prometheus.yml","/etc/prometheus/prometheus.yaml"}),"Prometheus config from which to remove managed job");if err:=fs.Parse(args);err!=nil{return err};if runtime.GOOS!="linux"||os.Geteuid()!=0{return errors.New("uninstall must run as root on Linux")};_ = run("systemctl","disable","--now","kata-exporter.service");if *prom!=""{if err:=removePrometheus(*prom);err!=nil{return err}};paths:=[]string{"/etc/systemd/system/kata-exporter.service","/usr/local/bin/kata-exporter"};if *purge{paths=append(paths,"/etc/kata-exporter","/etc/grafana/provisioning/dashboards/kata-exporter.yml","/var/lib/grafana/dashboards/kata-exporter.json")};for _,p:=range paths{if err:=os.RemoveAll(p);err!=nil{return err}};_ = run("systemctl","daemon-reload");if *prom!=""{_ = run("systemctl","reload","prometheus.service")};fmt.Println("Kata Exporter uninstalled.");return nil}

func patchPrometheus(path string)error{b,err:=os.ReadFile(path);if err!=nil{return fmt.Errorf("read Prometheus config: %w",err)};s:=string(b);if strings.Contains(s,managedBegin){return nil};lines:=strings.Split(s,"\n");at:=-1;indent:="";for i,line:=range lines{if strings.TrimSpace(line)=="scrape_configs:"{at=i;indent=line[:len(line)-len(strings.TrimLeft(line," \t"))];break}};if at<0{return fmt.Errorf("%s has no standalone scrape_configs key; refusing to modify it",path)};backup:=path+".kata-exporter.bak";if _,err:=os.Stat(backup);errors.Is(err,os.ErrNotExist){if err:=writeAtomic(backup,b,0644);err!=nil{return err}};snippet:=indentBlock(strings.TrimSpace(prometheusScrape),indent+"  ");out:=append([]string{},lines[:at+1]...);out=append(out,snippet);out=append(out,lines[at+1:]...);return writeAtomic(path,[]byte(strings.Join(out,"\n")),0644)}
func removePrometheus(path string)error{b,err:=os.ReadFile(path);if errors.Is(err,os.ErrNotExist){return nil};if err!=nil{return err};lines:=strings.Split(string(b),"\n");start,end:=-1,-1;for i,line:=range lines{switch strings.TrimSpace(line){case managedBegin:start=i;case managedEnd:if start>=0{end=i}}};if start<0||end<start{return nil};out:=append([]string{},lines[:start]...);out=append(out,lines[end+1:]...);return writeAtomic(path,[]byte(strings.Join(out,"\n")),0644)}
func indentBlock(s,indent string)string{lines:=strings.Split(s,"\n");for i:=range lines{lines[i]=indent+lines[i]};return strings.Join(lines,"\n")}
func prompt(r *bufio.Reader,label string)string{fmt.Printf("%s: ",label);s,_:=r.ReadString('\n');return strings.TrimSpace(s)}
func detect(paths []string)string{for _,p:=range paths{if _,err:=os.Stat(p);err==nil{return p}};return ""}
func detectDir(paths []string)string{for _,p:=range paths{if st,err:=os.Stat(p);err==nil&&st.IsDir(){return p}};return ""}
func copyFile(src,dst string,mode os.FileMode)error{in,err:=os.Open(src);if err!=nil{return err};defer in.Close();if err:=os.MkdirAll(filepath.Dir(dst),0755);err!=nil{return err};tmp:=dst+".tmp";out,err:=os.OpenFile(tmp,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,mode);if err!=nil{return err};_,cpErr:=io.Copy(out,in);closeErr:=out.Close();if cpErr!=nil{return cpErr};if closeErr!=nil{return closeErr};if err:=os.Chmod(tmp,mode);err!=nil{return err};return os.Rename(tmp,dst)}
func writeAtomic(path string,data []byte,mode os.FileMode)error{if err:=os.MkdirAll(filepath.Dir(path),0755);err!=nil{return err};tmp:=path+".tmp";if err:=os.WriteFile(tmp,data,mode);err!=nil{return err};if err:=os.Chmod(tmp,mode);err!=nil{return err};return os.Rename(tmp,path)}
func run(name string,args ...string)error{cmd:=exec.Command(name,args...);cmd.Stdout=os.Stdout;cmd.Stderr=os.Stderr;return cmd.Run()}
func ensureServiceUser()error{u,err:=user.Lookup("kata-exporter");if err!=nil{if err:=run("useradd","--system","--no-create-home","--home-dir","/nonexistent","--shell","/usr/sbin/nologin","kata-exporter");err!=nil{return fmt.Errorf("create kata-exporter user: %w",err)};u,err=user.Lookup("kata-exporter");if err!=nil{return err}};uid,err:=strconv.Atoi(u.Uid);if err!=nil{return err};gid,err:=strconv.Atoi(u.Gid);if err!=nil{return err};if err:=os.Chown("/etc/kata-exporter/config.json",uid,gid);err!=nil{return err};return os.Chmod("/etc/kata-exporter/config.json",0600)}

const systemdUnit=`[Unit]
Description=Prometheus exporter for SwitchBot Kata Friends
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=kata-exporter
Group=kata-exporter
ExecStart=/usr/local/bin/kata-exporter serve --config /etc/kata-exporter/config.json
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6
MemoryDenyWriteExecute=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
`
const grafanaProvider=`apiVersion: 1
providers:
  - name: Kata Exporter
    orgId: 1
    folder: Kata Friends
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: %s
`

// Ensure JSON remains linked in this package's tests/build even when only installer is used.
var _ = json.Valid
