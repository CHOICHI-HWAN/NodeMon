// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

const appsFixture = `{
 "system.cpu": {"dimensions": {"idle": {"value": 50}}},
 "apps.cpu": {"dimensions": {"nginx": {"value": 12.34}, "postgres": {"value": 80.5}, "idle-app": {"value": 0}, "node": {"value": 40}, "chrome": {"value": 20}, "sshd": {"value": 5}, "tiny": {"value": 0.05}}},
 "apps.mem": {"dimensions": {"nginx": {"value": 150}, "postgres": {"value": 900}, "idle-app": {"value": 5000}, "node": {"value": 300}, "chrome": {"value": 700}, "sshd": {"value": 3}, "tiny": {"value": 0.5}}}
}`

func names(a []appUse) string {
	var n []string
	for _, p := range a {
		n = append(n, p.Name)
	}
	return strings.Join(n, ",")
}

func TestAppsKeepTheBiggestCPUAndMemoryUsers(t *testing.T) {
	s := parse(t, appsFixture)
	// CPU: postgres 80.5, node 40, chrome 20, nginx 12.34 (4 kept); memory adds idle-app 5000 (the biggest), the other big ones are already in.
	if got := names(s.Apps); got != "postgres,node,chrome,nginx,idle-app" {
		t.Fatalf("apps = %s", got)
	}
	if s.Apps[0].CPU != 80.5 || s.Apps[0].Mem != 900 {
		t.Errorf("postgres = %+v", s.Apps[0])
	}
}

func TestNoAppsChartMeansNoApps(t *testing.T) {
	if s := parse(t, linuxFixture); len(s.Apps) != 0 {
		t.Fatalf("apps = %v", s.Apps)
	}
	if _, ok := record(parse(t, linuxFixture), nil)["ap"]; ok {
		t.Fatal("a computer without per-program data must not send an empty field")
	}
}

func TestEncodeAppsIsCompactAndSafe(t *testing.T) {
	got := encodeApps([]appUse{{"nginx", 12.34, 150.4}, {"we|ird:name", 1, 2}})
	if got != "nginx:12.3:150|we_ird_name:1:2" {
		t.Fatalf("encoded = %q", got)
	}
	long := strings.Repeat("가", 50)
	if n := len([]rune(strings.Split(encodeApps([]appUse{{long, 1, 1}}), ":")[0])); n != appName {
		t.Fatalf("name kept %d characters", n)
	}
}

func TestAveragingCountsAMissingProgramAsZero(t *testing.T) {
	var a agg
	a.add(sample{CPU: 1, Apps: []appUse{{"spike", 90, 100}, {"base", 10, 50}}})
	a.add(sample{CPU: 1, Apps: []appUse{{"base", 10, 50}}})
	m := a.mean()
	var spike appUse
	for _, p := range m.Apps {
		if p.Name == "spike" {
			spike = p
		}
	}
	if spike.CPU != 45 || spike.Mem != 50 { // busy in one of two samples
		t.Fatalf("spike = %+v", spike)
	}
}
