// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// status is the connector's status.json (written by nodemon-connector next to its saved credentials).
type status struct {
	Version    string `json:"version"`
	Updated    int64  `json:"updated"`
	LastSent   int64  `json:"last_sent"`
	Problem    string `json:"problem"`
	UploadSecs int    `json:"upload_secs"`
}

type state int

const (
	stateStarting state = iota // gray: no upload yet
	stateOK                    // green: uploads go through
	stateProblem               // amber: something is wrong but the connector is running
	stateStopped               // red: the connector is not running (or not set up)
)

// heartbeat is how long a status.json may go without a refresh before the connector counts as stopped. The
// connector refreshes it at least every 30 seconds while it runs.
const heartbeat = 2 * time.Minute

func readStatus(path string) (*status, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s status
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// evaluate turns the connector's status into the icon state and two lines of words for the menu.
func evaluate(s *status, readErr error, now time.Time) (state, string, string) {
	if readErr != nil || s == nil {
		return stateStopped, "Connector가 실행되지 않는 것 같습니다", "상태 파일이 없습니다. Connector를 먼저 실행해 주세요."
	}
	last := "아직 업로드 전"
	var sent time.Time
	if s.LastSent > 0 {
		sent = time.Unix(s.LastSent, 0)
		last = fmt.Sprintf("마지막 업로드 %s (%s)", sent.Local().Format("15:04"), ago(now.Sub(sent)))
	}
	if now.Sub(time.Unix(s.Updated, 0)) > heartbeat {
		return stateStopped, "Connector가 멈춘 것 같습니다", last
	}
	upload := time.Duration(s.UploadSecs) * time.Second
	switch {
	case s.Problem != "":
		return stateProblem, "문제 있음: " + s.Problem, last
	case sent.IsZero():
		return stateStarting, "시작하는 중", last
	case now.Sub(sent) > 2*upload+time.Minute:
		return stateProblem, "업로드가 늦어지고 있습니다", last
	}
	return stateOK, "정상", last
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "방금"
	case d < time.Hour:
		return fmt.Sprintf("%d분 전", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d시간 전", int(d.Hours()))
	}
	return fmt.Sprintf("%d일 전", int(d.Hours()/24))
}
