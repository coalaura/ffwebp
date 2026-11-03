package opts

type Common struct {
	Quality         int
	Lossless        bool
	OutputExtension string
	ICC             []byte
	Channels        int // Logical gray, gray-alpha, RGB or RGBA, independent of pixel storage.
}

func (c *Common) FillDefaults() {
	if c.Lossless {
		c.Quality = 100
	}
}
