package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// "golang.org/x/text/unicode/rangetable"

var dbug *log.Logger //nolint:gochecknoglobals

type Cursor interface {
	X(delta int) int
	Y(delta int) int
}

type cursor struct{ x, y, maxX, maxY int }

func (c *cursor) X(d int) int {
	c.x += d
	if c.x < 0 {
		c.x = 0
	}
	if c.x > c.maxX {
		c.x = c.maxX
	}
	return c.x
}

func (c *cursor) Y(d int) int {
	c.y += d
	if c.y < 0 {
		c.y = 0
	}
	if c.y > c.maxY {
		c.y = c.maxY
	}
	return c.y
}

type Gim struct {
	screen    tcell.Screen
	cursor    *cursor
	buffer    []rune
	movements map[rune]func([]rune, Cursor) (int, int)
}

func (g Gim) ShowCursor() {
	g.screen.ShowCursor(g.cursor.x, g.cursor.y)
}

var (
//	word = rangetable.Merge(unicode.L, unicode.N,
//
// &unicode.RangeTable{R16: []unicode.Range16{{Lo: '_', Hi: '_', Stride: 1}}, LatinOffset: 1})
)

//nolint:cyclop,funlen
func main() {
	defer func() {
		if r := recover(); r != nil {
			dbug.Println(r)
		}
	}()

	buf, err := setup()
	if err != nil {
		panic(err)
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	if err := screen.Init(); err != nil {
		panic(err)
	}
	w, h := screen.Size()
	gim := Gim{
		screen: screen,
		buffer: buf,
		cursor: &cursor{maxX: w - 1, maxY: h - 1},
		movements: map[rune]func([]rune, Cursor) (int, int){
			'j': func(_ []rune, c Cursor) (x, y int) { return c.X(0), c.Y(1) },
			'k': func(_ []rune, c Cursor) (x, y int) { return c.X(0), c.Y(-1) },
			'h': func(_ []rune, c Cursor) (x, y int) { return c.X(-1), c.Y(0) },
			'l': func(_ []rune, c Cursor) (x, y int) { return c.X(1), c.Y(0) },
		},
	}
	gim.ShowCursor()
	gim.screen.EnableMouse()
	gim.screen.SetStyle(tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorBlack.TrueColor()))
	if err != nil {
		panic(err)
	}
out:
	for {
		gim.draw()
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Fini()
			break out
			// gim.draw()
			// screen.Sync()
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'Z':
				ev := screen.PollEvent()
				if ek, ok := ev.(*tcell.EventKey); ok && ek.Rune() == 'Q' {
					screen.Fini()
					break out
				}
			default:
				fn, ok := gim.movements[ev.Rune()]
				if !ok {
					break
				}
				fn(buf, gim.cursor)
				dbug.Println(gim.cursor)
			}
		}
	}
}

func (g *Gim) draw() {
	g.screen.Clear()
	style := tcell.StyleDefault.Attributes(tcell.AttrBold).
		Foreground(tcell.ColorWhite.TrueColor()).Background(tcell.ColorBlack)
	for y, buf := range strings.Split(string(g.buffer), "\n") {
		for x, r := range buf {
			g.screen.SetContent(x, y, r, nil, style)
		}
	}
	g.ShowCursor()
	g.screen.Show()
}

func setup() ([]rune, error) {
	logf, err := os.Create("debug.log")
	dbug = log.New(logf, "", log.Llongfile|log.Ltime)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	return bytes.Runes(b), nil
}
