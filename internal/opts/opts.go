package opts

type Common struct {
	Quality  int
	Lossless bool
}

func (c *Common) FillDefaults() {
	if c.Quality == 0 {
		c.Quality = 85
	}

	if c.Lossless {
		c.Quality = 100
	}
}
