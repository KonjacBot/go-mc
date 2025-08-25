package bootstrap

import (
	"git.konjactw.dev/falloutBot/go-mc/data/registryid"
	"git.konjactw.dev/falloutBot/go-mc/level/block"
	"git.konjactw.dev/falloutBot/go-mc/registry"
)

func RegisterBlocks(reg *registry.Registry[block.Block]) {
	reg.Clear()
	for i, key := range registryid.Block {
		id, val := reg.Put(key, block.FromID[key])
		if int32(i) != id || val == nil || *val == nil {
			panic("register blocks failed")
		}
	}
}
