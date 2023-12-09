package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"

	"github.com/gdamore/tcell/v2"
)

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

	f, err := os.Open("utf8_demo.txt")
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}

	text := bytes.Split(b, []byte("\n"))
	rtext := make([][]rune, len(text))
	for i, line := range text {
		rtext[i] = bytes.Runes(line)
	}
	_, h := s.Size()
	buf := &Buffer{
		windowStart: 0,
		windowEnd:   uint(h),
		height: uint(h),
		fullText:    rtext,
	}

	for {
		s.Clear()

		for row, line := range buf.fullText[buf.windowStart:buf.windowEnd] {
			for col, r := range line {
				s.ShowCursor(buf.cursor.col, buf.cursor.row)
				s.SetContent(col+5, row, r, nil, style)
			}
		}

		s.Show()

		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyCtrlC:
				return
			case tcell.KeyDown:
				buf.UpdateRow(1)
			case tcell.KeyUp:
				buf.UpdateRow(-1)
			case tcell.KeyLeft:
				buf.cursor.col--
			case tcell.KeyRight:
				buf.cursor.col++
			}

			switch ev.Rune() {
			case 'j':
				buf.UpdateRow(1)
			case 'k':
				buf.UpdateRow(-1)
			case 'h':
				buf.cursor.col--
			case 'l':
				buf.cursor.col++
			}
		}
	}
}

type Cursor struct{ col, row int }

type Buffer struct {
	cursor Cursor
	windowStart uint
	windowEnd   uint
	height			uint
	fullText    [][]rune
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
