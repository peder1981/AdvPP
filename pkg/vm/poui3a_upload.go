package vm

import (
	"github.com/advpl/compiler/pkg/tools/shared"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.4 — PoUpload. O componente registra no shared o destino
// (cDiretorioDestino) com token opaco; o po-upload do browser faz POST
// multipart em /poui-upload?token=... e o (p-success) vira o evento
// "upload" -> OnUpload(aArquivos), aArquivos =
// {{cNome, cCaminhoGravado, nBytes}, ...}.

// poUploadReRegistra atualiza o spec do token com os setters correntes.
func poUploadReRegistra(c *poComp) {
	token, _ := c.props["token"].(string)
	dir, _ := c.props["dir"].(string)
	maxMB := 0
	switch n := c.props["maxSizeMB"].(type) {
	case float64:
		maxMB = int(n)
	case int:
		maxMB = n
	}
	exts := []string{}
	if arr, ok := c.props["extensions"].([]string); ok {
		exts = arr
	}
	mb := 10.0
	if f, ok := c.props["maxSizeMB"].(float64); ok && f > 0 {
		mb = f
	}
	restr := map[string]any{"maxFileSize": int64(mb * 1024 * 1024)}
	if len(exts) > 0 {
		restr["allowedExtensions"] = exts
	}
	c.app.setProp(c, "restrictions", restr)
	if token != "" {
		_, _ = shared.RegistrarUploadPoui(token, dir, maxMB, exts)
	}
}

// poUploadExts converte aExtensoes {{".png"}, {"jpg"}} ou {".png",".jpg"}
// em []string.
func poUploadExts(x advplrt.Value) []string {
	out := []string{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		out = append(out, advplrt.ToString(el))
	}
	return out
}

// po3aUploadMethod despacha os métodos do PoUpload.
func (v *VM) po3aUploadMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if c.typ != "upload" {
		return false, advplrt.Nil, nil
	}
	a := c.app
	switch method {
	case "SETEXTENSIONS":
		c.props["extensions"] = poUploadExts(getArg(args, 0))
		poUploadReRegistra(c)
	case "SETMAXSIZEMB":
		c.props["maxSizeMB"] = advplrt.ToFloat(getArg(args, 0))
		poUploadReRegistra(c)
	case "SETMULTIPLE":
		c.props["multiple"] = advplrt.ToBool(getArg(args, 0))
	case "ONUPLOAD":
		c.handlers["upload"] = getArg(args, 0)
	default:
		return false, advplrt.Nil, nil
	}
	_ = a
	return true, advplrt.Nil, nil
}

// poUploadToken registra o destino e devolve a URL do endpoint.
func poUploadToken(c *poComp) string {
	token := shared.NovoTokenUpload()
	c.props["token"] = token
	poUploadReRegistra(c)
	return "/poui-upload?token=" + token
}
