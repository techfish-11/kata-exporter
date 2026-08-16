package switchbot

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	token, secret, baseURL string
	http *http.Client
	now func() time.Time
	requests atomic.Uint64
	errors atomic.Uint64
}

type Device struct { DeviceID string `json:"deviceId"`; DeviceName string `json:"deviceName"`; DeviceType string `json:"deviceType"` }
type Status struct {
	DeviceID string `json:"deviceId"`; DeviceType string `json:"deviceType"`; Version string `json:"version"`
	Battery int `json:"battery"`; OnlineStatus string `json:"onlineStatus"`; Mode string `json:"mode"`; Status string `json:"status"`
	ChildLock string `json:"childLock"`; Hospitalized int `json:"hospitalized"`
}
type DiaryEvent struct { Event int `json:"event"`; Timestamp int64 `json:"timestamp"`; Detail string `json:"detail"` }
type AIDiary struct { Title string `json:"title"`; Date string `json:"date"`; Diary string `json:"diary"`; Timestamp int64 `json:"timestamp"` }
type ComicDiary struct { Title string `json:"title"`; Date string `json:"date"`; Diary string `json:"diary"`; ComicKey string `json:"comicKey"`; ThumbnailKey string `json:"thumbnailKey"`; Timestamp int64 `json:"timestamp"` }
type Diary struct { DeviceID string `json:"deviceId"`; DeviceType string `json:"deviceType"`; Diary []DiaryEvent `json:"diary"`; DiaryAI []AIDiary `json:"diaryAI"`; ComicDiaryAI []ComicDiary `json:"comicDiaryAI"` }

type envelope[T any] struct { StatusCode int `json:"statusCode"`; Message string `json:"message"`; Body T `json:"body"` }
type deviceListBody struct { DeviceList []Device `json:"deviceList"` }

func New(token, secret, baseURL string, timeout time.Duration) *Client {
	return &Client{token:token, secret:secret, baseURL:strings.TrimRight(baseURL,"/"), http:&http.Client{Timeout:timeout}, now:time.Now}
}

func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var out envelope[deviceListBody]
	if err := c.get(ctx, "/v1.1/devices", nil, &out); err != nil { return nil, err }
	var devices []Device
	for _, d := range out.Body.DeviceList { if d.DeviceType == "Kata Friends" { devices = append(devices, d) } }
	return devices, nil
}

func (c *Client) DeviceStatus(ctx context.Context, id string) (Status, error) {
	var out envelope[Status]
	err := c.get(ctx, "/v1.1/devices/"+url.PathEscape(id)+"/status", nil, &out)
	return out.Body, err
}

func (c *Client) Diary(ctx context.Context, id string, start, end time.Time) (Diary, error) {
	q := url.Values{"startTimestamp":{strconv.FormatInt(start.UnixMilli(),10)}, "endTimestamp":{strconv.FormatInt(end.UnixMilli(),10)}}
	var out envelope[Diary]
	err := c.get(ctx, "/v1.1/devices/"+url.PathEscape(id)+"/diary", q, &out)
	return out.Body, err
}

func (c *Client) Counts() (uint64,uint64) { return c.requests.Load(), c.errors.Load() }

func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	u := c.baseURL + path
	if len(q) > 0 { u += "?"+q.Encode() }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil { return err }
	timestamp := c.now().UnixMilli()
	nonce, err := randomNonce()
	if err != nil { return err }
	sign := Sign(c.token, c.secret, timestamp, nonce)
	req.Header.Set("Authorization", c.token)
	req.Header.Set("sign", sign)
	req.Header.Set("nonce", nonce)
	req.Header.Set("t", strconv.FormatInt(timestamp,10))
	req.Header.Set("Content-Type", "application/json")
	c.requests.Add(1)
	resp, err := c.http.Do(req)
	if err != nil { c.errors.Add(1); return fmt.Errorf("SwitchBot request: %w", err) }
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil { c.errors.Add(1); return err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { c.errors.Add(1); return fmt.Errorf("SwitchBot HTTP %d: %s", resp.StatusCode, compact(b)) }
	if err := json.Unmarshal(b, dst); err != nil { c.errors.Add(1); return fmt.Errorf("decode SwitchBot response: %w", err) }
	var status struct { StatusCode int `json:"statusCode"`; Message string `json:"message"` }
	_ = json.Unmarshal(b, &status)
	if status.StatusCode != 100 { c.errors.Add(1); return fmt.Errorf("SwitchBot API %d: %s", status.StatusCode, status.Message) }
	return nil
}

func Sign(token, secret string, timestamp int64, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = io.WriteString(mac, token+strconv.FormatInt(timestamp,10)+nonce)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func randomNonce() (string,error) { b:=make([]byte,16); if _,err:=rand.Read(b); err!=nil{return "",err}; return hex.EncodeToString(b),nil }
func compact(b []byte) string { s:=strings.Join(strings.Fields(string(b))," "); if len(s)>300{return s[:300]+"…"}; return s }

