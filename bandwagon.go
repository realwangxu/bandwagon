package bandwagon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"io/ioutil"
	"context"
	"errors"
)

const (
	defaultBaseURL = "https://api.64clouds.com/"
)

type Client struct {
	NewClient func() *http.Client
	BaseURL   string
	creds     Credentials
}

type Credentials struct {
	VeID   string `json:"veid"`
	APIKey string `json:"api_key"`
}

func (this *Credentials) Values() string {
	return fmt.Sprintf("veid=%v&api_key=%v", this.VeID, this.APIKey)
}

func makeClient() *http.Client {
	transport := &http.Transport{
		IdleConnTimeout:       5 * time.Second,
		DisableCompression:    true,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	}

	client := &http.Client{
		Transport: transport,
	}

	return client
}

func NewClient(cred Credentials) *Client {
	return &Client{
		NewClient: makeClient,
		BaseURL:   defaultBaseURL,
		creds:     cred,
	}
}

type Response struct {
	Error   int    `json:"error"`
	Message string `json:"message,omitempty"`
}

func (r *Response)String() string {
	return fmt.Sprintf("error: %v, message: %v", r.Error, r.Message)
}

type InfoVPS struct {
	VMType                          string            `json:"vm_type"`
	Hostname                        string            `json:"hostname"`
	NodeIP                          string            `json:"node_ip"`
	NodeAlias                       string            `json:"node_alias"`
	NodeLocation                    string            `json:"node_location"`
	NodeLocationID                  string            `json:"node_location_id"`
	NodeDatacenter                  string            `json:"node_datacenter"`
	LocationIPv6Ready               bool              `json:"location_ipv6_ready"`
	Plan                            string            `json:"plan"`
	PlanMonthlyData                 int64             `json:"plan_monthly_data"`
	MonthlyDataMultiplier           int64             `json:"monthly_data_multiplier"`
	PlanDisk                        int64             `json:"plan_disk"`
	PlanRam                         int             `json:"plan_ram"`
	PlanSwap                        int             `json:"plan_swap"`
	PlanMaxIPv6s                    int             `json:"plan_max_ipv6s"`
	Os                              string            `json:"os"`
	Email                           string            `json:"email"`
	DataCounter                     int64             `json:"data_counter"`
	DataNextReset                   int64             `json:"data_next_reset"`
	IpAddresses                     []string          `json:"ip_addresses"`
	PrivateIPAddresses              []string          `json:"private_ip_addresses"`
	IpNullRoutes                    []string          `json:"ip_nullroutes"`
	Iso1                            []byte            `json:"iso1"`
	Iso2                            []byte            `json:"iso2"`
	AvailableIsos                   []string          `json:"available_isos"`
	PlanPrivateNetworkAvailable     bool              `json:"plan_private_network_available"`
	LocationPrivateNetworkAvailable bool              `json:"location_private_network_available"`
	RdnsApiAvailable                bool              `json:"rdns_api_available"`
	Ptr                             map[string]string `json:"ptr"`
	Suspended                       bool              `json:"suspended"`
	PolicyViolation                 bool              `json:"policy_violation"`
	SuspensionCount                 bool              `json:"suspension_count"`
	TotalAbusePoints                int32             `json:"total_abuse_points"`
	MaxAbusePoints                  int32             `json:"max_abuse_points"`
	Error                           int32             `json:"error"`
}

func (info *InfoVPS)String() string {
	return fmt.Sprintf("IP Address: %v, Bandwidth Usage: %v GB, Reset time: %v", info.Ipv4(), info.DataCounter/1024/1024/1024, info.ResetTime())
}

func (this *InfoVPS) ResetTime() string {
	tn := time.Unix(this.DataNextReset, 0)
	return tn.Format("2006-01-02 15:04:05")
}

func (this *InfoVPS) Ipv4() string {
	if this.IpAddresses == nil || len(this.IpAddresses) <= 0 {
		return ""
	}
	return fmt.Sprintf("%v", this.IpAddresses[0])
}

func (this *Client) get(req *http.Request, ctx context.Context) (buf []byte, err error) {
	client := makeClient()
	r, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return
	}

	defer r.Body.Close()
	return ioutil.ReadAll(r.Body)
}

func (this *Client) do(req *http.Request, count int) ([]byte, error) {
	msgQ := make(chan string)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	for i := 0; i < count; i++ {
		go func() {
			if b, err := this.get(req, ctx); err == nil {
				cancel()
				msgQ <- string(b)
			}
		}()
	}

	select {
	case <-time.After(3000 * time.Millisecond):
		cancel()
		return nil, errors.New("time out i/o")
	case msg := <-msgQ:
		return []byte(msg), nil
	}
}

func (this *Client) Do(req *http.Request) (b []byte, err error) {
	return this.do(req, 20)
}

func (this *Client)httpGet(reqURL string) (res *Response, err error) {
	var req *http.Request = nil
	if req, err = http.NewRequest(http.MethodGet, reqURL, nil); err != nil {
		return nil, err
	}
	resp, err := this.Do(req)
	if err != nil {
		return nil, err
	}
	res = &Response{}
	err = json.NewDecoder(bytes.NewBuffer(resp)).Decode(res)
	return
}

func (this *Client) Info() (info *InfoVPS, err error) {
	reqURL := fmt.Sprintf("%v/v1/getServiceInfo?%v", defaultBaseURL, this.creds.Values())
	var req *http.Request = nil
	if req, err = http.NewRequest(http.MethodGet, reqURL, nil); err != nil {
		return nil, err
	}
	resp, err := this.Do(req)
	if err != nil {
		return nil, err
	}
	info = &InfoVPS{}
	err = json.NewDecoder(bytes.NewBuffer(resp)).Decode(info)
	return
}

func (this *Client) Start() (res *Response, err error) {
	return this.httpGet(fmt.Sprintf("%v/v1/start?%v", defaultBaseURL, this.creds.Values()))
}

func (this *Client) Stop() (res *Response, err error) {
	return this.httpGet(fmt.Sprintf("%v/v1/stop?%v", defaultBaseURL, this.creds.Values()))
}

func (this *Client) Kill() (res *Response, err error) {
	return this.httpGet(fmt.Sprintf("%v/v1/kill?%v", defaultBaseURL, this.creds.Values()))
}

func (this *Client) Reboot() (res *Response, err error) {
	return this.httpGet(fmt.Sprintf("%v/v1/restart?%v", defaultBaseURL, this.creds.Values()))
}

func (this *Client) Command(command string) (res *Response, err error) {
	return this.httpGet(fmt.Sprintf("%v/v1/basicShell/exec?command=%v&%v", defaultBaseURL, command, this.creds.Values()))
}
