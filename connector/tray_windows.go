// SPDX-License-Identifier: GPL-3.0-or-later
//go:build windows

package main

// The tray icon talks to Windows directly (user32, shell32, gdi32) so the connector keeps using only the
// Go standard library. With --tray the console window goes away, the log goes to connector.log next to the
// saved credentials, and an icon next to the clock shows how things are going.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

const trayAvailable = true

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procGetConsoleWindow       = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList  = kernel32.NewProc("GetConsoleProcessList")
	procFreeConsole            = kernel32.NewProc("FreeConsole")
	procCreateMutexW           = kernel32.NewProc("CreateMutexW")
	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procSetTimer               = user32.NewProc("SetTimer")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	procDestroyIcon            = user32.NewProc("DestroyIcon")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW          = shell32.NewProc("ShellExecuteW")
	procCreateBitmap           = gdi32.NewProc("CreateBitmap")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

const (
	wmNull             = 0x0000
	wmQueryEndSession  = 0x0011
	wmEndSession       = 0x0016
	wmTimer            = 0x0113
	wmLButtonUp        = 0x0202
	wmRButtonUp        = 0x0205
	wmTrayCallback     = 0x8000 + 1 // WM_APP + 1
	nimAdd             = 0
	nimModify          = 1
	nimDelete          = 2
	nifMessage         = 0x1
	nifIcon            = 0x2
	nifTip             = 0x4
	mfString           = 0x0
	mfGrayed           = 0x1
	mfSeparator        = 0x800
	tpmReturnCmd       = 0x100
	tpmRightButton     = 0x2
	tpmBottomAlign     = 0x20
	smCxSmIcon         = 49
	mbIconError        = 0x10
	errorAlreadyExists = 183

	menuDashboard = 1
	menuLog       = 2
	menuQuit      = 3
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type point struct{ x, y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type notifyIconData struct {
	size            uint32
	hwnd            uintptr
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            uintptr
	tip             [128]uint16
	state           uint32
	stateMask       uint32
	info            [256]uint16
	version         uint32
	infoTitle       [64]uint16
	infoFlags       uint32
	guidItem        [16]byte
	balloonIcon     uintptr
}

type iconInfo struct {
	isIcon   int32
	xHotspot uint32
	yHotspot uint32
	mask     uintptr
	color    uintptr
}

// tray is the icon and its hidden owner window. All of it runs on the one OS thread that created the window.
type tray struct {
	r          *runner // nil when the connector could not start; the icon then shows why
	startErr   string
	dashboard  string
	logPath    string
	upload     time.Duration
	hwnd       uintptr
	icon       uintptr
	iconState  int
	taskbarMsg uint32
	nid        notifyIconData
}

func runTray(o options) {
	runtime.LockOSThread() // the window and its message loop must stay on one thread

	// A second copy (the task started again, or started by hand) would upload everything twice.
	name, _ := syscall.UTF16PtrFromString(`Local\NodeMonConnectorTray`)
	if _, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name))); err == syscall.Errno(errorAlreadyExists) {
		os.Exit(0)
	}
	hideOwnConsole()
	logPath := filepath.Join(filepath.Dir(o.cfgPath), "connector.log")
	openLogFile(logPath)
	log.Printf("nodemon-connector %s starting in the tray", version)

	t := &tray{dashboard: o.dashboard, logPath: logPath, upload: o.upload, iconState: -1}
	r, err := newRunner(o)
	if err != nil {
		// Keep the icon up instead of exiting: the task would restart it every minute and nobody would see why.
		log.Print(err)
		t.startErr = err.Error()
	} else {
		t.r = r
		go r.loop(o.interval)
	}
	if err := t.create(); err != nil {
		log.Print(err)
		messageBox("NodeMon 아이콘을 만들지 못했습니다: " + err.Error())
		os.Exit(1)
	}
	t.run()
}

// hideOwnConsole closes the console window when this process is the only one using it (started by the task
// at logon). Started from a terminal that someone is using, the terminal is left alone.
func hideOwnConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	var pids [2]uint32
	if n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), 2); n > 1 {
		return
	}
	procShowWindow.Call(hwnd, 0) // SW_HIDE
	procFreeConsole.Call()
}

// openLogFile sends the log to a file, starting a fresh one once it passes 1 MB (the previous one is kept as .1).
func openLogFile(path string) {
	if fi, err := os.Stat(path); err == nil && fi.Size() > 1<<20 {
		os.Rename(path, path+".1")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

func (t *tray) create() error {
	inst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("NodeMonConnectorTray")
	wc := wndClassEx{wndProc: syscall.NewCallback(t.wndProc), instance: inst, className: className}
	wc.size = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}
	// A hidden top-level window rather than a message-only one: only top-level windows hear "TaskbarCreated",
	// which is how the icon comes back after Explorer restarts.
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0, 0, inst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	t.hwnd = hwnd
	taskbar, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	m, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(taskbar)))
	t.taskbarMsg = uint32(m)

	t.nid = notifyIconData{hwnd: hwnd, id: 1, flags: nifMessage | nifIcon | nifTip, callbackMessage: wmTrayCallback}
	t.nid.size = uint32(unsafe.Sizeof(t.nid))
	t.refresh()
	if ok, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid))); ok == 0 {
		return fmt.Errorf("Shell_NotifyIcon: %w", err)
	}
	procSetTimer.Call(hwnd, 1, 5000, 0) // keep the tooltip's "minutes ago" and the color current
	return nil
}

func (t *tray) run() {
	var m msg
	for {
		if r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0); int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (t *tray) wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmTrayCallback:
		if lp := uint32(lParam) & 0xffff; lp == wmRButtonUp || lp == wmLButtonUp {
			t.showMenu()
		}
		return 0
	case wmTimer:
		t.refresh()
		procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&t.nid)))
		return 0
	case wmQueryEndSession:
		return 1
	case wmEndSession:
		if wParam != 0 { // Windows is signing out or shutting down: keep the usage total
			t.quit()
		}
		return 0
	case t.taskbarMsg:
		procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid)))
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func (h *health) get() (time.Time, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastSent, h.problem
}

// status describes the current state in words, and which icon color goes with it.
func (t *tray) status() (state int, lines []string) {
	if t.r == nil {
		return stateError, []string{"시작하지 못했습니다", t.startErr}
	}
	sent, problem := t.r.health.get()
	last := "아직 업로드 전"
	if !sent.IsZero() {
		last = fmt.Sprintf("마지막 업로드 %s (%s)", sent.Local().Format("15:04"), ago(time.Since(sent)))
	}
	switch {
	case problem != "":
		return stateProblem, []string{"문제 있음: " + problem, last}
	case sent.IsZero():
		return stateStarting, []string{"시작하는 중", last}
	case time.Since(sent) > 2*t.upload+time.Minute:
		return stateProblem, []string{"업로드가 늦어지고 있습니다", last}
	}
	return stateOK, []string{"정상", last}
}

func ago(d time.Duration) string {
	if d < time.Minute {
		return "방금"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d분 전", int(d.Minutes()))
	}
	return fmt.Sprintf("%d시간 전", int(d.Hours()))
}

// refresh updates the tooltip and, when the state changed, the icon color.
func (t *tray) refresh() {
	state, lines := t.status()
	tip := "NodeMon · " + lines[0]
	if len(lines) > 1 {
		tip += "\n" + lines[1]
	}
	copyUTF16(t.nid.tip[:], tip)
	if state != t.iconState {
		if icon := makeIcon(stateColors[state]); icon != 0 {
			old := t.icon
			t.icon, t.iconState, t.nid.icon = icon, state, icon
			if old != 0 {
				procDestroyIcon.Call(old)
			}
		}
	}
}

func (t *tray) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	_, lines := t.status()
	for _, l := range lines {
		appendMenu(menu, mfString|mfGrayed, 0, l)
	}
	appendMenu(menu, mfSeparator, 0, "")
	if t.dashboard != "" {
		appendMenu(menu, mfString, menuDashboard, "대시보드 열기")
	}
	appendMenu(menu, mfString, menuLog, "로그 보기")
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, menuQuit, "종료")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(t.hwnd) // otherwise the menu does not close when clicking elsewhere
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton|tpmBottomAlign, uintptr(pt.x), uintptr(pt.y), 0, t.hwnd, 0)
	procPostMessageW.Call(t.hwnd, wmNull, 0, 0)
	switch cmd {
	case menuDashboard:
		shellOpen(t.dashboard)
	case menuLog:
		shellOpen(t.logPath)
	case menuQuit:
		log.Print("stopped from the tray icon")
		t.quit()
	}
}

func (t *tray) quit() {
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
	if t.r != nil {
		t.r.stop()
	}
	os.Exit(0)
}

func appendMenu(menu, flags, id uintptr, text string) {
	p, _ := syscall.UTF16PtrFromString(truncate(text, 120))
	procAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(p)))
}

func shellOpen(target string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	p, _ := syscall.UTF16PtrFromString(target)
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, 1) // SW_SHOWNORMAL
}

func messageBox(text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString("NodeMon")
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbIconError)
}

// copyUTF16 writes s into a fixed-size, NUL-terminated buffer, cutting it short if it does not fit.
func copyUTF16(dst []uint16, s string) {
	u, _ := syscall.UTF16FromString(truncate(s, len(dst)/2))
	if len(u) > len(dst) {
		u = append(u[:len(dst)-1], 0)
	}
	copy(dst, u)
	for i := len(u); i < len(dst); i++ {
		dst[i] = 0
	}
}

func truncate(s string, runes int) string {
	if utf8.RuneCountInString(s) <= runes {
		return s
	}
	r := []rune(s)
	return string(r[:runes-1]) + "…"
}

const (
	stateStarting = iota
	stateOK
	stateProblem
	stateError
)

// stateColors are 0xRRGGBB: gray while starting, green when uploads go through, amber on a problem, red when stopped.
var stateColors = map[int]uint32{stateStarting: 0x8a8f98, stateOK: 0x2e9e5b, stateProblem: 0xe0a100, stateError: 0xd64545}

// makeIcon draws a rounded square in the given color with three white bars, at the size Windows uses for tray icons.
func makeIcon(rgb uint32) uintptr {
	n, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	size := int(n)
	if size < 16 {
		size = 16
	}
	px := drawIcon(size, rgb)
	color, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 32, uintptr(unsafe.Pointer(&px[0])))
	mask := make([]byte, (size+15)/16*2*size) // all zero: the alpha channel decides what shows
	maskBmp, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	defer procDeleteObject.Call(color)
	defer procDeleteObject.Call(maskBmp)
	ii := iconInfo{isIcon: 1, mask: maskBmp, color: color}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return icon
}

// drawIcon returns size×size pixels as 0xAARRGGBB, top row first.
func drawIcon(size int, rgb uint32) []uint32 {
	px := make([]uint32, size*size)
	bg := 0xff000000 | rgb
	const white = 0xffffffff
	radius := float64(size) / 5
	inside := func(x, y int) bool { // within the rounded square
		fx, fy := float64(x)+0.5, float64(y)+0.5
		cx := min(max(fx, radius), float64(size)-radius)
		cy := min(max(fy, radius), float64(size)-radius)
		return (fx-cx)*(fx-cx)+(fy-cy)*(fy-cy) <= radius*radius
	}
	pad := max(size/5, 3)
	inner := size - 2*pad
	barW := max(inner/4, 2)
	gap := (inner - 3*barW) / 2
	heights := []float64{0.45, 0.75, 1.0}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !inside(x, y) {
				continue
			}
			px[y*size+x] = bg
			for i, h := range heights {
				left := pad + i*(barW+gap)
				top := size - pad - int(float64(inner)*h+0.5)
				if x >= left && x < left+barW && y >= top && y < size-pad {
					px[y*size+x] = white
				}
			}
		}
	}
	return px
}
