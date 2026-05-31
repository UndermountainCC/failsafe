// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package mode

// Chain is a list of mode sources tried in order. First to return a value wins;
// if all skip, Default is used.
type Chain struct {
	Sources []Source
	Default string
}

// Resolve walks Sources in order and returns (value, source-that-resolved, nil)
// for the first hit, or (Default, nil, nil) if all sources skip.
func (c Chain) Resolve(env map[string]string) (string, Source, error) {
	for _, s := range c.Sources {
		v, ok, err := s.Resolve(env)
		if err != nil {
			return "", nil, err
		}
		if ok {
			return v, s, nil
		}
	}
	return c.Default, nil, nil
}

// FirstWritable returns the first source that is Writable() and whose Path()
// resolves in the current env. Used by the toggle/mode subcommands.
func (c Chain) FirstWritable(env map[string]string) (Source, string, bool) {
	for _, s := range c.Sources {
		if !s.Writable() {
			continue
		}
		path, ok := s.Path(env)
		if !ok {
			continue
		}
		return s, path, true
	}
	return nil, "", false
}
