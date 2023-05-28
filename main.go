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
	"golang.org/x/text/unicode/rangetable"
)

var dbug *log.Logger
var GimWord *unicode.RangeTable

func init() {
	f, _ := os.Create("debug.log")
	dbug = log.New(f, "", log.Llongfile|log.Ltime)
}

func init() {
	runes := make([]rune, 26*2+1)
	for i := 0; i < 26; i++ {
		runes[i] = rune('a' + i)
	}
	for i := 0; i < 26; i++ {
		runes[i+25] = rune('A' + i)
	}
	runes[len(runes)-1] = '_'
	GimWord = rangetable.Merge(unicode.L, unicode.N,
		// '_' character
		&unicode.RangeTable{R16: []unicode.Range16{{Lo: 95, Hi: 95, Stride: 1}}, LatinOffset: 1},
	)
	dbug.Printf("%+v\n", GimWord)
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
		X, Y  int
		virtX int
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
			Cursor: Cursor{Y: h / 2},
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
	g.ShowCursor(g.Buffers[0].Cursor.X, g.Buffers[0].Cursor.Y)
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
			case 'h', 'j', 'k', 'l', '^', '$', 'g', 'G', 'w', 'W':
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

func getWordAtIndex(line Line, index int) string {
	// Check if the index is within the valid range of the line
	if index < 0 || index >= len(line) {
		return ""
	}

	// Find the start and end indices of the word
	startIndex := index
	endIndex := index

	// TODO(jay): Update this to "word"
	for startIndex > 0 && !unicode.IsSpace(line[startIndex-1]) {
		startIndex--
	}

	// TODO(jay): Update this to "word"
	for endIndex < len(line)-1 && !unicode.IsSpace(line[endIndex+1]) {
		endIndex++
	}
	return string(line[startIndex : endIndex+1])
}

func (b *Buffer) UpdateWindow(r rune) {
	isAtEdge := func(y int) bool {
		return !(y >= topEdge && y < b.Height)
	}
	endCol := func() int { return len(*b.Window()[b.Cursor.Y]) - 1 }
	switch r {
	case 'w':
		// When on "White space" character what do?
		// When on "Punct/Symbol" character what do?
		// When on "Alnum_" character what do?
		line := *b.Window().Line(b.Cursor.Y)
		if b.X >= len(line) {
			b.Cursor.X = 0
			b.Cursor.virtX = 0
			b.Cursor.Y++
			if isAtEdge(b.Cursor.Y) {
				b.Cursor.Y--
				b.Bounds.Top++
				b.Bounds.Base++
				if b.Bounds.Base > len(b.Text) {
					b.Bounds.Top--
					b.Bounds.Base = len(b.Text)
					b.Cursor.X = len(*b.Text.Line(len(b.Text) - 1))
					b.Cursor.virtX = len(*b.Text.Line(len(b.Text) - 1))
				}
				break
			}
			break
		}
		var nextIdx, nextWordIdx int
		dbug.Printf("Indexing: %q\n", string(line[b.Cursor.X:]))
		switch {
		case unicode.IsSpace(line[b.X]):
			nextIdx = strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
				return !unicode.IsSpace(r)
			})
		case !unicode.Is(GimWord, line[b.X]):
			dbug.Println("Why aren't we making it in here?")
			nextIdx = strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
				dbug.Printf("!gimword, is gim word %s: %t\n", string(r), unicode.Is(GimWord, r))
				return unicode.Is(GimWord, r) || unicode.IsSpace(r)
			})
			if nextIdx != -1 && unicode.IsSpace(line[b.Cursor.X+nextIdx]) {
				nextIdx += strings.IndexFunc(string(line[b.Cursor.X+nextIdx:]), func(r rune) bool {
					return !unicode.IsSpace(r)
				})
				dbug.Printf("Indexing after space: %q\n", string(line[nextIdx+b.Cursor.X:]))
			}
		default:
			nextIdx = strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
				dbug.Printf("is gim word %s: %t\n", string(r), unicode.Is(GimWord, r))
				return !unicode.Is(GimWord, r)
			})
			dbug.Printf("Indexing: %q\n", string(line[nextIdx+b.Cursor.X:]))
			dbug.Println("Non index:", nextIdx, "Next index:", nextWordIdx)
			if nextIdx != -1 && unicode.IsSpace(line[b.Cursor.X+nextIdx]) {
				nextIdx += strings.IndexFunc(string(line[b.Cursor.X+nextIdx:]), func(r rune) bool {
					return !unicode.IsSpace(r)
				})
				dbug.Printf("Indexing after space: %q\n", string(line[nextIdx+b.Cursor.X:]))
			}
		}
		word := getWordAtIndex(line, b.Cursor.X)
		dbug.Println("Current word: ", word)
		if nextIdx == -1 {
			b.Cursor.X = 0
			b.Cursor.virtX = 0
			b.Cursor.Y++
			if isAtEdge(b.Cursor.Y) {
				b.Cursor.Y--
				b.Bounds.Top++
				b.Bounds.Base++
				if b.Bounds.Base > len(b.Text) {
					b.Bounds.Top--
					b.Bounds.Base = len(b.Text)
					b.Cursor.X = len(*b.Text.Line(len(b.Text) - 1))
					b.Cursor.virtX = len(*b.Text.Line(len(b.Text) - 1))
				}
				break
			}
			break
		}
		b.Cursor.X += nextIdx + nextWordIdx
		b.Cursor.virtX += nextIdx + nextWordIdx
	case 'W':
		line := *b.Window().Line(b.Cursor.Y)
		word := getWordAtIndex(line, b.Cursor.X)
		dbug.Println("Current word: ", word)
		wsIdx := strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
			return unicode.IsSpace(r)
		})
		dbug.Printf("Indexing: %q\n", string(line[b.Cursor.X:]))
		if wsIdx == -1 {
			b.Cursor.X = 0
			b.Cursor.virtX = 0
			b.Cursor.Y++
			if isAtEdge(b.Cursor.Y) {
				b.Cursor.Y--
				b.Bounds.Top++
				b.Bounds.Base++
				if b.Bounds.Base > len(b.Text) {
					b.Bounds.Top--
					b.Bounds.Base = len(b.Text)
					b.Cursor.X = len(*b.Text.Line(len(b.Text) - 1))
					b.Cursor.virtX = len(*b.Text.Line(len(b.Text) - 1))
				}
				break
			}
			break
		}
		dbug.Printf("Indexing: %q\n", string(line[wsIdx+b.Cursor.X:]))
		nextWordIdx := strings.IndexFunc(string(line[wsIdx+b.Cursor.X:]), func(r rune) bool {
			return !unicode.IsSpace(r)
		})
		dbug.Println("Non index:", wsIdx, "Next index:", nextWordIdx)
		b.Cursor.X += wsIdx + nextWordIdx
		b.Cursor.virtX += wsIdx + nextWordIdx
	case 'g':
		b.Cursor.Y = topEdge
		b.Bounds.Top = 0
		b.Bounds.Base = b.Height
	case 'G':
		b.Cursor.Y = b.Height - 1
		b.Bounds.Top = len(b.Text) - len(b.Window())
		b.Bounds.Base = len(b.Text)
	case '^':
		b.Cursor.X = strings.IndexFunc(string(*b.Window()[b.Cursor.Y]), func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsNumber(r)
		})
		if b.Cursor.X == -1 {
			b.Cursor.X = 0
		}
		b.Cursor.virtX = b.X
	case '$':
		b.Cursor.X = endCol()
		if b.Cursor.X == -1 {
			b.Cursor.X = 0
		}
		b.Cursor.virtX = b.Cursor.X
	case 'h':
		if b.Cursor.X == 0 {
			break
		}
		b.Cursor.virtX--
		b.Cursor.X--
	case 'j':
		b.Cursor.Y++
		if isAtEdge(b.Cursor.Y) {
			b.Cursor.Y--
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
		b.Cursor.Y--
		if isAtEdge(b.Cursor.Y) {
			b.Cursor.Y++
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
		if b.Cursor.X > endCol() {
			break
		}
		b.Cursor.virtX++
		b.Cursor.X++
	}
	dbug.Printf("%+v and %+v, endCol: %+v\n", b.Bounds, b.Cursor, endCol())
}

func (c *Cursor) UpdateX(col int) {
	c.X = col
	if c.X < 0 {
		c.X = 0
	}
	if c.virtX <= col {
		c.X = c.virtX
	}
}
