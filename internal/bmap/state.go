package bmap

// FromState is the inverse of ToYAML in state mode: it reads every field,
// read-only ones included, at its path, and applies the forward transforms
// (env and secret maps to lists, item aliases to API names). nil stays nil.
// Terraform needs it to compare a plan with prior state in API terms.
func (k *Kind) FromState(doc *Map) (*Map, error) {
	out := NewMap()
	for _, f := range k.Fields {
		value := get(doc, f.Path)
		if value == absent {
			continue
		}
		if value == nil {
			out.Set(f.Name, nil)
			continue
		}
		mapped, err := mapValue(value, f, false)
		if err != nil {
			return nil, err
		}
		out.Set(f.Name, mapped)
	}
	return out, nil
}
