package extension

import (
	"math/rand"

	"github.com/hipoint-airpress/airpress/template"
	"github.com/hipoint-airpress/airpress/util"
)

type toolExtension struct {
	Template *template.Template
}

func RegisterToolFunc(template *template.Template) {
	t := &toolExtension{
		Template: template,
	}
	t.addRainbow()
	t.addRandom()
}

// addRainbow 彩虹分页算法
func (t *toolExtension) addRainbow() {
	t.Template.AddFunc("rainbowPage", util.RainbowPage)
}

func (t *toolExtension) addRandom() {
	random := func(min, max int) int {
		return min + rand.Intn(max-min)
	}
	t.Template.AddFunc("random", random)
}
