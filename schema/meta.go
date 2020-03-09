package schema

import (
	"github.com/chirino/graphql/internal/assets"
	"io/ioutil"
)

var Meta *Schema

func init() {
	Meta = &Schema{} // bootstrap
	Meta = New()
	file, err := assets.FileSystem.Open("/meta.graphql")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	if err := Meta.Parse(string(data)); err != nil {
		panic(err)
	}
}
