package opts

type Common struct {
	Quality         int
	Lossless        bool
	OutputExtension string
	ICC             []byte
}

func (c *Common) FillDefaults() {
	if c.Lossless {
		c.Quality = 100
	}
}
