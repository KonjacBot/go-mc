package command

import (
	"io"
	"unsafe"

	pk "git.konjactw.dev/falloutBot/go-mc/net/packet"
)

const (
	isExecutable = 1 << (iota + 2)
	hasRedirect
	hasSuggestionsType
)

func (g *Graph) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Array(g.nodes),
		pk.VarInt(0),
	}.WriteTo(w)
}

func (n Node) WriteTo(w io.Writer) (int64, error) {
	var flag byte
	flag |= n.kind & 0x03
	if n.Run != nil {
		flag |= isExecutable
	}
	return pk.Tuple{
		pk.Byte(flag),
		pk.Array((*[]pk.VarInt)(unsafe.Pointer(&n.Children))),
		pk.Opt{
			Has:   func() bool { return n.kind&hasRedirect != 0 },
			Field: nil, // TODO: send redirect node
		},
		pk.Opt{
			Has:   func() bool { return n.kind == ArgumentNode || n.kind == LiteralNode },
			Field: pk.String(n.Name),
		},
		pk.Opt{
			Has:   func() bool { return n.kind == ArgumentNode },
			Field: n.Parser, // Parser identifier and Properties
		},
		pk.Opt{
			Has:   func() bool { return flag&hasSuggestionsType != 0 },
			Field: nil, // TODO: send Suggestions type
		},
	}.WriteTo(w)
}

func (g *Graph) ReadFrom(r io.Reader) (int64, error) {
	var totalBytes int64

	// 讀取 nodes 數組
	nodesReader := pk.Array(&g.nodes)
	n, err := nodesReader.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += n

	// 讀取版本號 (VarInt(0))
	var version pk.VarInt
	n, err = version.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += n

	return totalBytes, nil
}

func (n *Node) ReadFrom(r io.Reader) (int64, error) {
	var totalBytes int64

	// 讀取 flag
	var flag pk.Byte
	bytes, err := flag.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += bytes

	// 從 flag 中提取 kind
	n.kind = byte(flag) & 0x03

	// 讀取 Children 數組
	childrenSlice := (*[]pk.VarInt)(unsafe.Pointer(&n.Children))
	childrenReader := pk.Array(childrenSlice)
	bytes, err = childrenReader.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += bytes

	// 讀取 redirect 選項
	redirectOpt := pk.Opt{
		Has:   func() bool { return n.kind&hasRedirect != 0 },
		Field: nil, // TODO: 實現 redirect node 讀取
	}
	bytes, err = redirectOpt.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += bytes

	// 讀取名稱選項 (ArgumentNode 或 LiteralNode)
	if n.kind == ArgumentNode || n.kind == LiteralNode {
		var nameStr pk.String
		nameOpt := pk.Opt{
			Has:   func() bool { return n.kind == ArgumentNode || n.kind == LiteralNode },
			Field: &nameStr,
		}
		bytes, err = nameOpt.ReadFrom(r)
		if err != nil {
			return totalBytes, err
		}
		totalBytes += bytes
		n.Name = string(nameStr)
	}

	// 讀取 Parser 選項 (僅 ArgumentNode)
	if n.kind == ArgumentNode {
		parserOpt := pk.Opt{
			Has:   func() bool { return n.kind == ArgumentNode },
			Field: &n.Parser,
		}
		bytes, err = parserOpt.ReadFrom(r)
		if err != nil {
			return totalBytes, err
		}
		totalBytes += bytes
	}

	// 讀取 Suggestions 類型選項
	suggestionsOpt := pk.Opt{
		Has:   func() bool { return byte(flag)&hasSuggestionsType != 0 },
		Field: nil, // TODO: 實現 Suggestions 類型讀取
	}
	bytes, err = suggestionsOpt.ReadFrom(r)
	if err != nil {
		return totalBytes, err
	}
	totalBytes += bytes

	if byte(flag)&isExecutable != 0 {
	}

	return totalBytes, nil
}
