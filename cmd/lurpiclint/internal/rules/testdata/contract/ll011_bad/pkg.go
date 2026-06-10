package ll011_bad

type BadFacet struct {
	facet.Facet
	ch chan int
}

func newBad() *BadFacet {
	f := &BadFacet{ch: make(chan int)}
	f.Facet = facet.NewFacet()

	go func() {
		f.ch <- 42
	}()

	val := <-f.ch
	_ = val

	return f
}
