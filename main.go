package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strconv"

	"github.com/gdamore/tcell/v2"
)

type Gim struct {
	screen  tcell.Screen
	focus   *Buffer
	buffers []*Buffer
	style   tcell.Style
	mode    Mode
	run     bool
}

type Mode uint8

func (g *Gim) Handle() {
	switch g.mode {
	case ModeNormal:
		switch ev := g.screen.PollEvent().(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyCtrlC:
				g.run = false
			case tcell.KeyDown:
				g.focus.UpdateRow(1)
			case tcell.KeyUp:
				g.focus.UpdateRow(-1)
			case tcell.KeyLeft:
				g.focus.cursor.col--
			case tcell.KeyRight:
				g.focus.cursor.col++
			}

			switch ev.Rune() {
			case 'j':
				g.focus.UpdateRow(1)
			case 'k':
				g.focus.UpdateRow(-1)
			case 'h':
				g.focus.cursor.col--
			case 'l':
				g.focus.cursor.col++
			case 'i':
				g.mode = ModeInsert
				g.screen.SetCursorStyle(tcell.CursorStyleBlinkingBar)
			}
		}
	case ModeInsert:
		switch ev := g.screen.PollEvent().(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyESC {
				g.mode = ModeNormal
				g.screen.SetCursorStyle(tcell.CursorStyleSteadyBlock)
				return
			}
			g.focus.Insert(ev.Rune())
		}
	}
}

const (
	ModeNormal Mode = iota
	ModeInsert
)

func (g Gim) Draw() {
	g.screen.Clear()

	g.screen.ShowCursor(g.focus.cursor.col, g.focus.cursor.row)

	for row, line := range g.focus.Window() {
		start := strconv.Itoa(row + 1 + int(g.focus.windowStart))
		padding := len(strconv.Itoa(len(g.focus.fullText)))
		for i := 0; i < padding; i++ {
			r := ' '
			if diff := padding - len(start) - i; diff < 1 {
				r = rune(start[-diff])
			}
			g.screen.SetContent(i, row, r, nil, g.style)
		}
		g.screen.SetContent(padding, row, tcell.RuneVLine, nil, g.style.Dim(true))
		for col, r := range line {
			style := g.style
			switch r {
			case ' ':
				r = tcell.RuneBullet
				style = style.Dim(true)
			case '	':
				r = tcell.RuneRArrow
				style = style.Dim(true)
			}
			g.screen.SetContent(col+4, row, r, nil, style)
		}
	}

	g.screen.Show()
}

func main() {
	s, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	defer func(s tcell.Screen) {
		s.Fini()
		if err := recover(); err != nil {
			slog.Error("A Fatal error was caught", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}(s)

	if err := s.Init(); err != nil {
		panic(err)
	}

	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	s.SetStyle(style)

	var f *os.File
	for i, arg := range os.Args {
		switch i {
		case 0:
			f, err = os.Open("utf8_demo.txt")
			if err != nil {
				panic(err)
			}
		case 1:
			f, err = os.Open(arg)
			if err != nil {
				f, err = os.Create(arg)
				if err != nil {
					panic(err)
				}
			}
		}
	}
	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}

	text := bytes.Split(b, []byte("\n"))
	rtext := make([][]rune, len(text)-1)
	for i, line := range text[:len(text)-1] {
		rtext[i] = bytes.Runes(line)
	}
	_, h := s.Size()
	buf := NewBuffer(h, rtext)
	gim := Gim{run: true, screen: s, focus: buf, buffers: []*Buffer{buf}, style: style}

	for gim.run {
		gim.Draw()

		gim.Handle()
	}
}

type Cursor struct{ col, row int }

type Buffer struct {
	cursor      Cursor
	windowStart uint
	windowEnd   uint
	height      uint
	fullText    [][]rune
}

func NewBuffer(height int, text [][]rune) *Buffer {
	return &Buffer{
		cursor:      Cursor{col: 5, row: 0},
		windowStart: 0,
		windowEnd:   uint(min(height, len(text))),
		height:      uint(height),
		fullText:    text,
	}
}

func (b Buffer) Insert(r rune) {
	b.fullText[uint(b.cursor.row)+b.windowStart][b.cursor.col] = r
}

func (b Buffer) Window() [][]rune {
	return b.fullText[b.windowStart:b.windowEnd]
}

func (b *Buffer) UpdateRow(by int) {
	b.cursor.row += by
	switch {
	case by < 0:
		if b.cursor.row < int(b.height/2) && b.windowStart > 0 {
			b.cursor.row++
			b.windowStart--
			b.windowEnd--
		}
	case by > 0:
		if b.cursor.row > int(b.height/2) && b.windowEnd < uint(len(b.fullText)) {
			b.cursor.row--
			b.windowStart++
			b.windowEnd++
		}
	}
}
