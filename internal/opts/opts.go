package opts

type Common struct {
	Quality         int
	Lossless        bool
	OutputExtension string
}

func (c *Common) FillDefaults() {
	if c.Lossless {
		c.Quality = 100
	}
}
