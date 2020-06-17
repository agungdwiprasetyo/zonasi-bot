package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"time"
)

const (
	lineBotAPI   = "https://api.agungdwiprasetyo.com/linebot/v1/bot/pushmessage"
	targetSource = "https://lampung.siap-ppdb.com/seleksi/reguler/sma/1-31080149-1000.json"
)

var latestNoUrut float64

type param struct {
	studentName      string
	notifTo          string
	basicAuthLineBot string
}

func httpRequest(method, url string, body io.Reader, header map[string]string) (respBody []byte, err error) {
	var req *http.Request
	var resp *http.Response

	req, err = http.NewRequest(method, url, body)
	defer func() {
		if req != nil {
			req.Close = true
			if req.Body != nil {
				req.Body.Close()
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	for key, value := range header {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, _ = ioutil.ReadAll(resp.Body)
	defer func() {
		resp.Close = true
		client.CloseIdleConnections()
		resp.Body.Close()
	}()

	return
}

func fetchZonasi(req *param) {
	defer func() { recover() }()

	log.Println("fetch")

	body, err := httpRequest("GET", targetSource,
		nil, nil)
	if err != nil {
		panic(err)
	}

	var response struct {
		Data [][]interface{} `json:"data"`
	}
	json.Unmarshal(body, &response)

	for _, ss := range response.Data {
		name := ss[2].(string)
		no := ss[0].(float64)
		if name == req.studentName && latestNoUrut != no {
			fmt.Println(latestNoUrut, no)
			latestNoUrut = no
			notif(req)
		}
	}
}

func notif(req *param) {
	var payload = fmt.Sprintf(`{
		"to": "%s",
		"title": "no urut zonasi %s",
		"message": "%s : %v"
	}`, req.notifTo, req.studentName, req.studentName, latestNoUrut)

	resp, _ := httpRequest(
		"POST",
		lineBotAPI,
		bytes.NewBuffer([]byte(payload)),
		map[string]string{"Authorization": "Basic " + req.basicAuthLineBot},
	)
	log.Println(req.basicAuthLineBot, string(resp))
}

type schedulerJob struct {
	Name   string
	Ticker *time.Ticker

	f       interface{}
	handler reflect.Value
	params  []reflect.Value
}

func (j *schedulerJob) Do() {
	j.handler.Call(j.params)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	var interval int64
	var pr param
	flag.StringVar(&pr.studentName, "studentName", "JUANA PUSPANINGTYAS", "set student name")
	flag.StringVar(&pr.notifTo, "notifTo", "", "set user id line for target notification")
	flag.StringVar(&pr.basicAuthLineBot, "basicAuthLineBot", "", "set basic auth key")
	flag.Int64Var(&interval, "interval", 2, "set interval, in second")

	flag.Parse()

	jobs := []*schedulerJob{
		{
			Name: "ZONASI",
			f:    fetchZonasi,
			params: []reflect.Value{
				reflect.ValueOf(&pr),
			},
		},
	}

	var activeJobs []*schedulerJob
	var schedulerChannels []reflect.SelectCase
	for _, job := range jobs {
		job.handler = reflect.ValueOf(job.f)
		if job.handler.Kind() != reflect.Func {
			continue
		}

		job.Ticker = time.NewTicker(time.Duration(interval) * time.Second)
		activeJobs = append(activeJobs, job)
		schedulerChannels = append(schedulerChannels, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(job.Ticker.C)})
	}

	if len(activeJobs) == 0 {
		panic("no job actived")
	}

	for {
		chosen, _, ok := reflect.Select(schedulerChannels)
		if !ok {
			continue
		}

		job := activeJobs[chosen]
		job.Do()
	}
}
