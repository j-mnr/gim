package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/text/unicode/rangetable"
)

const (
	xbones = 9760 // ☠️
)

var dbug *log.Logger //nolint:gochecknoglobals

type (
	// Buffer is a temporary area of memory that stores the text of a file.
	Buffer struct {
		Text [][]rune
		Bounds
		Cursor
		Height int
	}

	// Bounds represent the visual area that can be seen on a [tcell.Screen].
	Bounds struct{ Top, Base, Left, Right int }

	// Cursor is a indicator used to show the current position on a [tcell.Screen].
	Cursor struct {
		X, Y int
	}
)

//nolint:cyclop
func main() {
	scr, f, err := setup()
	if err != nil {
		panic(err)
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
			case 'h', 'j', 'k', 'l', '^', '$', 'g', 'G', 'w', 'W', 'b', 'B':

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

func (b Buffer) Window() [][]rune { return b.Text[b.Top:b.Base] }

type Gim struct {
	Word *unicode.RangeTable
	tcell.Screen
	Buffers []Buffer
}

func bytesToLine(bytes []byte) []rune {
	runes := make([]rune, 0, len(bytes))
	for len(bytes) > 0 {
		runeValue, runeSize := utf8.DecodeRune(bytes)
		runes = append(runes, runeValue)
		bytes = bytes[runeSize:]
	}
	return runes
}

func NewGim(s tcell.Screen, texts []io.Reader) *Gim {
	gim := &Gim{
		Screen: s,
		Word: rangetable.Merge(unicode.L, unicode.N,
			&unicode.RangeTable{R16: []unicode.Range16{{Lo: '_', Hi: '_', Stride: 1}}, LatinOffset: 1}),
	}
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
		var contents [][]rune
		scnr := bufio.NewScanner(r)
		for scnr.Scan() {
			contents = append(contents, bytesToLine(scnr.Bytes()))
		}
		bufs[i] = Buffer{
			Text:   contents,
			Bounds: Bounds{Base: len(contents)},
			Height: h,
		}
	}
	gim.Buffers = bufs
	return gim
}

// Line is a string of text in a file without an ending newline character.
type Line []rune

func (l Line) Rune(index int) rune {
	if len(l) <= index {
		return xbones
	}
	return l[index]
}

func draw(g *Gim) {
	g.Clear()
	style := tcell.StyleDefault.Attributes(tcell.AttrBold).
		Foreground(tcell.ColorWhite.TrueColor()).Background(tcell.ColorBlack)
	for y, line := range g.Buffers[0].Window() {
		for x, r := range line {
			g.SetContent(x, y, r, nil, style)
		}
	}
	g.ShowCursor(g.Buffers[0].Cursor.X, g.Buffers[0].Cursor.Y)
	g.Show()
}

func setup() (tcell.Screen, io.Reader, error) {
	logf, err := os.Create("debug.log")
	dbug = log.New(logf, "", log.Llongfile|log.Ltime)
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}
	scr, err := tcell.NewScreen()
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}
	return scr, f, nil
}
