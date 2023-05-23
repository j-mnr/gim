package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

var dbug *log.Logger

func init() {
	f, _ := os.Create("debug.log")
	dbug = log.New(f, "", log.Llongfile|log.Ltime)
}

type (
	// Buffer is a temporary area of memory that stores the text of a file.
	Buffer struct {
		Text Text
		Bounds
		Cursor
	}

	// Bounds represent the visual area that can be seen on a [tcell.Screen].
	Bounds struct{ Top, Base, Left, Right int }

	// Cursor is a indicator used to show the current position on a [tcell.Screen].
	Cursor struct {
		style tcell.CursorStyle
		Spot
	}
)

func (b Buffer) Window() Text { return b.Text[b.Top:b.Base] }

type Gim struct {
	tcell.Screen
	Buffers []Buffer
}

func bytesToLine(bytes []byte) *Line {
	runes := make(Line, 0, len(bytes))
	for len(bytes) > 0 {
		runeValue, runeSize := utf8.DecodeRune(bytes)
		runes = append(runes, runeValue)
		bytes = bytes[runeSize:]
	}
	return &runes
}

func NewGim(s tcell.Screen, texts []io.Reader) *Gim {
	gim := &Gim{Screen: s}
	if err := gim.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	gim.ShowCursor(0, 0)
	gim.EnableMouse()
	defStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack.TrueColor()).
		Foreground(tcell.ColorWhite)
	gim.SetStyle(defStyle)

	_, h := gim.Size()
	bufs := make([]Buffer, len(texts))
	for i, r := range texts {
		var contents Text
		scnr := bufio.NewScanner(r)
		for scnr.Scan() {
			contents = append(contents, bytesToLine(scnr.Bytes()))
		}
		bufs[i] = Buffer{
			Text:   contents,
			Bounds: Bounds{Base: h},
			Cursor: Cursor{Spot: Spot{Y: h / 2}},
		}
	}
	gim.Buffers = bufs
	return gim
}

type Text []*Line

func (t Text) Line(index int) *Line {
	if len(t) <= index {
		return nil
	}
	return t[index]
}

// Line is a string of text in a file without an ending newline character.
type Line []rune

func (l Line) Rune(index int) rune {
	if len(l) <= index {
		return 9760 // ☠️
	}
	return l[index]
}

// Spot represents a cartesian coordinate on a plane.
type Spot struct{ X, Y int }

func draw(g *Gim) {
	g.Clear()
	style := tcell.StyleDefault.Attributes(tcell.AttrBlink | tcell.AttrBold).
		Foreground(tcell.ColorBlack.TrueColor()).Background(tcell.ColorBlack)
	for y, line := range g.Buffers[0].Window() {
		for x, r := range *line {
			g.SetContent(x, y, r, nil, style)
		}
	}
	g.ShowCursor(g.Buffers[0].Cursor.Spot.X, g.Buffers[0].Cursor.Spot.Y)
	g.Show()
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	scr, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	gim := NewGim(scr, []io.Reader{f})
	style := tcell.StyleDefault.Foreground(tcell.ColorCadetBlue.TrueColor()).
		Background(tcell.ColorBlack)

	for y, line := range gim.Buffers[0].Text[:1] {
		if len(*line) == 0 {
			gim.SetContent(0, y, ' ', nil, style)
		}
		for x, r := range *line {
			gim.SetContent(x, y, r, nil, style)
		}
	}

	for {
		draw(gim)
		switch ev := gim.PollEvent().(type) {
		case *tcell.EventResize:
			gim.Sync()
			draw(gim)
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'h', 'j', 'k', 'l':
				gim.Buffers[0].UpdateWindow(ev.Rune())
			case 'Z':
				ev := gim.PollEvent()
				if ek, ok := ev.(*tcell.EventKey); ok && ek.Rune() == 'Q' {
					gim.Fini()
					os.Exit(0)
				}
			}
		}
	}
}

func (b Buffer) Line(bufno int) Line {
	return *b.Text.Line(b.Cursor.Spot.Y)
}

func (b *Buffer) UpdateWindow(r rune) {
	hasReachedCriticalMass := func(y int) bool {
		return !(y >= 0 && y <= 53)
	}
	switch r {
	case 'h':
		if b.Cursor.Spot.X == 0 {
			break
		}
		b.Cursor.Spot.X--
	case 'j':
		b.Cursor.Spot.Y++
		if hasReachedCriticalMass(b.Cursor.Y) {
			b.Cursor.Spot.Y--
			b.Bounds.Top++
			b.Bounds.Base++
			if b.Bounds.Base > len(b.Text) {
				b.Bounds.Top--
				b.Bounds.Base = len(b.Text)
			}
			break
		}
		if endCol := len(b.Line(0)) - 1; b.Cursor.Spot.X > endCol {
			if endCol < 0 {
				b.Cursor.Spot.X = 0
				break
			}
			b.Cursor.Spot.X = endCol
		}
	case 'k':
		b.Cursor.Spot.Y--
		if hasReachedCriticalMass(b.Cursor.Y) {
			b.Cursor.Spot.Y++
			b.Bounds.Top--
			b.Bounds.Base--
			if b.Bounds.Top < 0 {
				b.Bounds.Base++
				b.Bounds.Top = 0
			}
			break
		}
		if endCol := len(b.Line(0)) - 1; b.Cursor.Spot.X > endCol {
			if endCol < 0 {
				b.Cursor.Spot.X = 0
				break
			}
			b.Cursor.Spot.X = endCol
		}
	case 'l':
		if b.Cursor.Spot.X > len(b.Line(0))-1 {
			break
		}
		b.Cursor.Spot.X++
	}
}
