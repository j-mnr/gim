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

const fill = 'x'

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
			Bounds: Bounds{Base: len(contents)},
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
			case 'h', 'j', 'k', 'l', '^', '$', 'g', 'G', 'w', 'W', 'b', 'B':
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

func (b Buffer) CP() rune {
	return (*b.Window().Line(b.Y))[b.X]
}

func KeyB(line []rune) (index int) {
	if len(line) == 0 {
		return 0
	}
	n := len(line) - 1
	switch {
	case unicode.IsSpace(line[n]):
		nonWSIdx := strings.LastIndexFunc(string(line), func(r rune) bool {
			return !unicode.IsSpace(r)
		})
		index = strings.LastIndexFunc(string(line[:nonWSIdx]), func(r rune) bool {
			return unicode.IsSpace(r)
		})
	case unicode.Is(GimWord, line[n]):
	default: // Any other code point that's not white space.
		// Special case: Start of word needs to go back to last word.
		if n+1 >= 2 && unicode.IsSpace(line[n-1]) && !unicode.IsSpace(line[n]) {
			return KeyShiftB(line[:n])
		}
		index = strings.LastIndexFunc(string(line), func(r rune) bool {
			return unicode.IsSpace(r)
		})
		dbug.Printf("line: %s\n", string(line))
		dbug.Println("index:", index)
	}
	return index + 1
}

// KeyShiftB finds the index in the given line that would get you one WORD back from the end of the
// line. If no such WORD exists -1 is returned as a sentinel value.
func KeyShiftB(line []rune) (index int) {
	if len(line) <= 1 {
		return -1
	}
	s, n := string(line), len(line)-1
	switch {
	case unicode.IsSpace(line[n]):
		wsIdx := strings.LastIndexFunc(s, func(r rune) bool {
			return unicode.IsSpace(r)
		})
		if wsIdx == -1 {
			return wsIdx
		}
		nonWSIdx := strings.LastIndexFunc(s[:wsIdx], func(r rune) bool {
			return !unicode.IsSpace(r)
		})
		if nonWSIdx == -1 {
			return nonWSIdx
		}
		for i := nonWSIdx; i >= 0; i-- {
			if unicode.IsSpace(line[i]) {
				break
			}
			index = i
		}
	case unicode.IsSpace(line[n-1]):
		return KeyShiftB(line[:n])
	default:
		for i := n; i >= 0; i-- {
			if unicode.IsSpace(line[i]) {
				break
			}
			index = i
		}
	}
	return index
}

func (b *Buffer) UpdateWindow(r rune) {
	isPastBound := func(y int) bool {
		return y < topEdge || y >= b.Height
	}
	endCol := func() int { return len(*b.Window()[b.Cursor.Y]) - 1 }
	line := *b.Window().Line(b.Cursor.Y)
	switch r {
	case 'b':
	case 'B':
		if len(line) == 0 {
			b.Cursor.Y--
			if isPastBound(b.Cursor.Y) {
				b.Cursor.Y++
				b.Bounds.Top--
				b.Bounds.Base--
				if b.Bounds.Top < 0 {
					b.Bounds.Top = 0
					b.Bounds.Base++
					b.Cursor.X, b.Cursor.virtX = 0, 0
					break
				}
			}
			n := KeyShiftB(append(*b.Window().Line(b.Y), fill))
			if n == -1 {
				n = 0
			}
			b.X, b.virtX = n, n
			break
		}
		i := KeyShiftB((*b.Window().Line(b.Y))[:b.X+1])
		if i == -1 {
			b.Cursor.Y--
			if isPastBound(b.Y) {
				b.Cursor.Y++
				b.Bounds.Top--
				b.Bounds.Base--
				if b.Bounds.Top < 0 {
					b.Bounds.Top = 0
					b.Bounds.Base++
					b.Cursor.X, b.Cursor.virtX = 0, 0
					break
				}
			}
			n := KeyShiftB(append(*b.Window().Line(b.Y), fill))
			if n == -1 {
				n = 0
			}
			b.X, b.virtX = n, n
			break
		}
		b.Cursor.X = i
		b.virtX = i
	case 'w':
		// keyW(*b.Window().Line(b.Y))
		if b.X >= len(line) {
			b.Cursor.X = 0
			b.Cursor.virtX = 0
			b.Cursor.Y++
			if isPastBound(b.Cursor.Y) {
				b.Cursor.Y--
				b.Bounds.Top++
				b.Bounds.Base++
				if b.Bounds.Base > len(b.Text) {
					b.Bounds.Top--
					b.Bounds.Base = len(b.Text)
					b.Cursor.X = len(*b.Text.Line(len(b.Text) - 1))
					b.Cursor.virtX = len(*b.Text.Line(len(b.Text) - 1))
				}
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
			nextIdx = strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
				return unicode.Is(GimWord, r) || unicode.IsSpace(r)
			})
			if nextIdx != -1 && unicode.IsSpace(line[b.Cursor.X+nextIdx]) {
				nextIdx += strings.IndexFunc(string(line[b.Cursor.X+nextIdx:]), func(r rune) bool {
					return !unicode.IsSpace(r)
				})
			}
		default:
			nextIdx = strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
				return !unicode.Is(GimWord, r)
			})
			if nextIdx != -1 && unicode.IsSpace(line[b.Cursor.X+nextIdx]) {
				nextIdx += strings.IndexFunc(string(line[b.Cursor.X+nextIdx:]), func(r rune) bool {
					return !unicode.IsSpace(r)
				})
			}
		}
		if nextIdx == -1 {
			b.Cursor.X = 0
			b.Cursor.virtX = 0
			b.Cursor.Y++
			if isPastBound(b.Cursor.Y) {
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
		wsIdx := strings.IndexFunc(string(line[b.Cursor.X:]), func(r rune) bool {
			return unicode.IsSpace(r)
		})
		if wsIdx == -1 {
			b.X, b.virtX = 0, 0
			b.Y++
			if isPastBound(b.Cursor.Y) {
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
		if isPastBound(b.Cursor.Y) {
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
		if isPastBound(b.Cursor.Y) {
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
