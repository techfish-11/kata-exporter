package switchbot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSign(t *testing.T){got:=Sign("token","secret",1700000000000,"nonce");want:="Ho/pm1Q6hyf9kroxzCu/cSBo7lGKad4tesq6eb2CpUg=";if got!=want{t.Fatalf("Sign()=%q want %q",got,want)}}

func TestDevicesFiltersKata(t *testing.T){srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Header.Get("Authorization")!="t"{t.Error("missing auth")};w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"statusCode":100,"message":"success","body":{"deviceList":[{"deviceId":"k1","deviceName":"Kata","deviceType":"Kata Friends"},{"deviceId":"m1","deviceName":"Meter","deviceType":"Meter"}]}}`))}));defer srv.Close();c:=New("t","s",srv.URL,time.Second);d,err:=c.Devices(context.Background());if err!=nil{t.Fatal(err)};if len(d)!=1||d[0].DeviceID!="k1"{t.Fatalf("unexpected devices: %#v",d)}}
