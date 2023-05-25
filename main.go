package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode"
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
		Height int
	}

	// Bounds represent the visual area that can be seen on a [tcell.Screen].
	Bounds struct{ Top, Base, Left, Right int }

	// Cursor is a indicator used to show the current position on a [tcell.Screen].
	Cursor struct {
		style tcell.CursorStyle
		Spot
		virt Spot
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
	gim.SetStyle(tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorBlack.TrueColor()))
	bufs := make([]Buffer, len(texts))
	_, h := gim.Size()
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
			Height: h,
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
	style := tcell.StyleDefault.Attributes(tcell.AttrBold).
		Foreground(tcell.ColorWhite.TrueColor()).Background(tcell.ColorBlack)
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
	for {
		draw(gim)
		switch ev := gim.PollEvent().(type) {
		case *tcell.EventResize:
			_, h := gim.Size()
			for i := range gim.Buffers {
				gim.Buffers[i].Height = h
				if gim.Buffers[i].Y > h {
					gim.Buffers[i].Y = h - 1
				}
				gim.Buffers[i].Base = h
			}

			dbug.Println("A RESIZE EVENT HAS OCCURRED!!!! ", h)
			b := gim.Buffers[0]
			dbug.Printf("%+v and %+v\n", b.Bounds, b.Cursor)
			draw(gim)
			gim.Sync()
		case *tcell.EventKey:
			switch ev.Rune() {
			// Movement keys
			case 'h', 'j', 'k', 'l', '^', '$', 'g', 'G':
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

const (
	topEdge = 0
)

func (b *Buffer) UpdateWindow(r rune) {
	isAtEdge := func(y int) bool {
		return !(y >= topEdge && y < b.Height)
	}
	endCol := func() int { return len(*b.Window()[b.Cursor.Spot.Y]) - 1 }
	switch r {
	case 'g':
		b.Cursor.Y = topEdge
		b.Bounds.Top = 0
		b.Bounds.Base = b.Height
	case 'G':
		b.Cursor.Y = b.Height - 1
		b.Bounds.Top = len(b.Text) - len(b.Window())
		b.Bounds.Base = len(b.Text)
	case '^':
		b.Cursor.Spot.X = strings.IndexFunc(string(*b.Window()[b.Cursor.Spot.Y]), func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsNumber(r)
		})
		if b.Cursor.Spot.X == -1 {
			b.Cursor.Spot.X = 0
		}
	case '$':
		b.Cursor.Spot.X = endCol()
		if b.Cursor.Spot.X == -1 {
			b.Cursor.Spot.X = 0
		}
	case 'h':
		if b.Cursor.Spot.X == 0 {
			break
		}
		b.Cursor.Spot.X--
	case 'j':
		b.Cursor.Spot.Y++
		if isAtEdge(b.Cursor.Y) {
			b.Cursor.Spot.Y--
			b.Bounds.Top++
			b.Bounds.Base++
			if b.Bounds.Base > len(b.Text) {
				b.Bounds.Top--
				b.Bounds.Base = len(b.Text)
			}
			break
		}
		b.Cursor.UpdateX(endCol())
	case 'k':
		b.Cursor.Spot.Y--
		if isAtEdge(b.Cursor.Y) {
			b.Cursor.Spot.Y++
			b.Bounds.Top--
			b.Bounds.Base--
			if b.Bounds.Top < 0 {
				b.Bounds.Base++
				b.Bounds.Top = 0
			}
			break
		}
		b.Cursor.UpdateX(endCol())
	case 'l':
		if b.Cursor.Spot.X > endCol() {
			break
		}
		b.Cursor.Spot.X++
	}
	dbug.Printf("%+v and %+v\n", b.Bounds, b.Cursor)
}

func (c *Cursor) UpdateX(col int) {
	if c.Spot.X <= col {
		return
	}
	c.Spot.X = col
	if c.Spot.X < 0 {
		c.Spot.X = 0
	}
}
