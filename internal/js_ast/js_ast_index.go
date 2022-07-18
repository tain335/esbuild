package js_ast

// 暂时动态生成的模板都以这个index开始，避免后面冲突
var index uint32 = 2294967295

func MakeDynamicIndex() uint32 {
	index++
	return index
}
