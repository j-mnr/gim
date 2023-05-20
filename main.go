package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gdamore/tcell/v2"
)

func display(s Gim, data []byte) {
	s.Clear()
	style := tcell.StyleDefault.Foreground(tcell.ColorCadetBlue.TrueColor()).Background(tcell.ColorBlack)
	x, y := 0, 0
	for _, b := range data {
		if b == '\n' {
			y++
			x = 0
			continue
		}
		s.SetContent(x, y, rune(b), nil, style)
		x++
	}
	s.Show()
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	scr, e := tcell.NewScreen()
	if e != nil {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}
	s := Gim{Screen: scr}
	if e := s.Init(); e != nil {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}
	s.ShowCursor(0, 0)

	s.EnableMouse()
	defStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorWhite)
	s.SetStyle(defStyle)

	for {
		display(s, b)
		switch ev := s.PollEvent().(type) {
		case *tcell.EventResize:
			s.Sync()
			display(s, b)
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'h', 'j', 'k', 'l':
				s.SetCursor(ev.Rune())
			case 'Z':
				ev := s.PollEvent()
				if ek, ok := ev.(*tcell.EventKey); ok && ek.Rune() == 'Q' {
					s.Fini()
					os.Exit(0)
				}
			}
			if ev.Key() == tcell.KeyEscape {
				s.Fini()
				os.Exit(0)
			}
		}
	}
}

type Gim struct {
	tcell.Screen
	cpos [2]uint16
}

func (s *Gim) SetCursor(r rune) {
	w, h := s.Size()
	switch r {
	case 'h':
		if s.cpos[0] == 0 {
			break
		}
		s.cpos[0]--
	case 'j':
		if int(s.cpos[1]) == h-1 {
			break
		}
		s.cpos[1]++
	case 'k':
		if s.cpos[1] == 0 {
			break
		}
		s.cpos[1]--
	case 'l':
		if int(s.cpos[0]) == w-1 {
			break
		}
		s.cpos[0]++
	}
	s.ShowCursor(int(s.cpos[0]), int(s.cpos[1]))
}
