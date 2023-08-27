package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type TruenasAlerts struct {
	Uuid   string `json:"uuid"`
	Source string `json:"source"`
	Klass  string `json:"klass"`
	Args   struct {
		Volume  string `json:"volume"`
		State   string `json:"state"`
		Status  string `json:"status"`
		Devices string `json:"devices"`
	} `json:"args"`
	Node     string `json:"node"`
	Key      string `json:"key"`
	Datetime struct {
		Date int64 `json:"$date"`
	} `json:"datetime"`
	LastOccurrence struct {
		Date int64 `json:"$date"`
	} `json:"last_occurrence"`
	Dismissed bool        `json:"dismissed"`
	Mail      interface{} `json:"mail"`
	Text      string      `json:"text"`
	Id        string      `json:"id"`
	Level     string      `json:"level"`
	Formatted string      `json:"formatted"`
	OneShot   bool        `json:"one_shot"`
}

type NtfyMessage struct {
	Topic    string        `json:"topic"`
	Message  string        `json:"message"`
	Title    string        `json:"title"`
	Tags     []string      `json:"tags,omitempty"`
	Priority uint8         `json:"priority"`
	Attach   string        `json:"attach,omitempty"`
	Filename string        `json:"filename,omitempty"`
	Click    string        `json:"click,omitempty"`
	Actions  []NtfyActions `json:"actions,omitempty"`
}

type NtfyActions struct {
	Action string `json:"action,omitempty"`
	Label  string `json:"label,omitempty"`
	Url    string `json:"url,omitempty"`
}

type NtfyResponse struct {
	Id       string   `json:"id"`
	Time     int      `json:"time"`
	Expires  int      `json:"expires"`
	Event    string   `json:"event"`
	Topic    string   `json:"topic"`
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Priority int      `json:"priority"`
	Tags     []string `json:"tags"`
	Actions  []struct {
		Id     string `json:"id"`
		Action string `json:"action"`
		Label  string `json:"label"`
		Clear  bool   `json:"clear"`
		Url    string `json:"url"`
	} `json:"actions"`
}

var (
	truenasUrl string
	apikey     string
	ntfyUrl    string
	ntfyTopic  string
	interval   int64
)

func getAlerts() ([]TruenasAlerts, error) {
	//slog.Info("url: " + truenasUrl)
	url := truenasUrl + "/api/v2.0/alert/list"
	var nasAlerts []TruenasAlerts
	//var currentAlerts []TruenasAlerts
	token := "Bearer " + apikey
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nasAlerts, err
	}
	request.Header.Set("accept", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", token)

	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return nasAlerts, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nasAlerts, err
	}

	err = json.Unmarshal(body, &nasAlerts)

	if err != nil {
		return nasAlerts, err
	}

	for i := 0; i < len(nasAlerts); i++ {
		if nasAlerts[i].Datetime.Date >= interval {
			nasAlerts = append(nasAlerts[:i], nasAlerts[i+1:]...)
		}
	}

	return nasAlerts, nil
}

func CreateNtfyMessage(alerts TruenasAlerts) NtfyMessage {
	alertTime := time.UnixMilli(alerts.Datetime.Date).Format("2006/01/02 15:04:05")
	lastOccTime := time.UnixMilli(alerts.LastOccurrence.Date).Format("2006/01/02 15:04:05")
	message := fmt.Sprintf(`
Level:%s
Time: %s
Last Occurence: %s
Dissmissed: %t
Message: %s`, alerts.Level, alertTime, lastOccTime, alerts.Dismissed, strings.Replace(strings.Replace(strings.Replace(alerts.Formatted, "<ul><li>", "\n", -1), "</li></ul>", "", -1), "<br>", "\n", -1))
	title := strings.Split(alerts.Formatted, ":")[0]
	var level string
	var priority uint8
	switch alerts.Level {
	case "INFO":
		level = "large_blue_circle"
		priority = 1
	case "NOTICE":
		level = "purple_circle"
		priority = 2
	case "WARNING":
		level = "yellow_circle"
		priority = 3
	case "ERROR", "ALERT":
		level = "orange_circle"
		priority = 4
	case "CRITICAL", "EMERGENCY":
		level = "red_circle"
		priority = 5
	}
	ntfyMessage := NtfyMessage{
		Topic:    ntfyTopic,
		Title:    title,
		Message:  message,
		Priority: priority,
		Tags:     []string{level, "TrueNas"},
		Click:    truenasUrl,
		Actions: []NtfyActions{
			{
				Action: "View",
				Label:  "Admin Panel",
				Url:    truenasUrl,
			},
		},
	}

	return ntfyMessage
}

func sendNtfy(content NtfyMessage) error {
	var ntfyResponse NtfyResponse
	ntfyBodyJson, err := json.Marshal(content)

	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", ntfyUrl, bytes.NewReader(ntfyBodyJson))
	//request.Header.Set("Content-Type", "application/json")
	if err != nil {
		return err
	}

	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil || response.StatusCode != 200 {
		return err
	}

	err = json.Unmarshal(body, &ntfyResponse)

	if err != nil {
		return err
	}

	return nil
}

func main() {
	currentTime := time.Now()
	truenasUrl = os.Getenv("TRUENASURL")
	apikey = os.Getenv("APIKEY")
	ntfyUrl = os.Getenv("NTFYURL")
	ntfyTopic = os.Getenv("TOPIC")
	duration, err := time.ParseDuration(os.Getenv("INTERVAL"))
	if err != nil {
		slog.Error("unable to parse time", err)
	}
	interval = currentTime.Add(-duration).UnixMilli()

	//slog.Info("sending to " + ntfyUrl + " with topic: " + ntfyTopic)
	alerts, err := getAlerts()
	if err != nil {
		slog.Error("unable to fetch alerts from TrueNas", err)
	}
	slog.Info(fmt.Sprintf("Found %d alerts!", len(alerts)))
	for _, alert := range alerts {
		message := CreateNtfyMessage(alert)
		err = sendNtfy(message)
		if err != nil {
			slog.Error("Unable to send to ntfy at the moment", err)
		}
	}
}
