package acceso

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	infra "uniapp/internal/infra"
	modelo "uniapp/internal/models/acceso"
)

func TestVinculacionApoderadoYEscolar(t *testing.T) {
	h, bus := servidor(t)
	var token string
	bus.Registrar(modelo.VinculacionSolicitada, infra.NuevoObservadorFunc("captura-vinculacion-http", func(e modelo.Event) {
		token, _ = e.Payload["token"].(string)
	}))

	apoderado := ingresar(t, h, "apoderado.prueba@example.test")
	escolar := ingresar(t, h, "escolar.prueba@example.test")

	w := llamar(h, "POST", "/api/apoderado/vinculaciones", `{"email_escolar":"escolar.prueba@example.test"}`, apoderado)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"estado":"pendiente"`) {
		t.Fatalf("solicitud: %d %s", w.Code, w.Body)
	}
	if token == "" {
		t.Fatal("debía emitirse el token de vinculación")
	}

	w = llamar(h, "POST", "/api/escolar/vinculaciones/consultar", `{"token":"`+token+`"}`, escolar)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Apoderado Prueba") {
		t.Fatalf("consulta: %d %s", w.Code, w.Body)
	}

	w = llamar(h, "POST", "/api/escolar/vinculaciones/aceptar", `{"token":"`+token+`"}`, escolar)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"estado":"aceptada"`) {
		t.Fatalf("aceptar: %d %s", w.Code, w.Body)
	}

	w = llamar(h, "POST", "/api/escolar/vinculaciones/aceptar", `{"token":"`+token+`"}`, escolar)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("token de un solo uso: %d %s", w.Code, w.Body)
	}

	w = llamar(h, "GET", "/api/escolar/apoderado", "", escolar)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "apoderado.prueba@example.test") {
		t.Fatalf("mi apoderado: %d %s", w.Code, w.Body)
	}

	w = llamar(h, "GET", "/api/apoderado/vinculaciones", "", apoderado)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "escolar.prueba@example.test") {
		t.Fatalf("listado: %d %s", w.Code, w.Body)
	}
}

func TestAutorizacionDeNuevasReservas(t *testing.T) {
	h, bus := servidor(t)
	var token string
	bus.Registrar(modelo.VinculacionSolicitada, infra.NuevoObservadorFunc("captura-autorizacion-http", func(e modelo.Event) {
		token, _ = e.Payload["token"].(string)
	}))

	apoderado := ingresar(t, h, "apoderado.prueba@example.test")
	escolar := ingresar(t, h, "escolar.prueba@example.test")
	tutor := ingresar(t, h, "tutor.prueba@uni.edu.pe")

	if w := llamar(h, "POST", "/api/apoderado/vinculaciones", `{"email_escolar":"escolar.prueba@example.test"}`, apoderado); w.Code != http.StatusAccepted {
		t.Fatalf("solicitud: %d %s", w.Code, w.Body)
	}
	if w := llamar(h, "POST", "/api/escolar/vinculaciones/aceptar", `{"token":"`+token+`"}`, escolar); w.Code != http.StatusOK {
		t.Fatalf("aceptar: %d %s", w.Code, w.Body)
	}

	var vinculacion map[string]any
	if err := json.Unmarshal(llamar(h, "GET", "/api/escolar/apoderado", "", escolar).Body.Bytes(), &vinculacion); err != nil {
		t.Fatal(err)
	}
	escolarID := vinculacion["vinculacion"].(map[string]any)["escolar"].(map[string]any)["id"].(string)
	ruta := "/api/apoderado/escolares/" + escolarID + "/autorizacion"

	w := llamar(h, "POST", ruta, `{"autorizado":false}`, apoderado)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"estado":"no_autorizado"`) {
		t.Fatalf("revocar: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"creada_en"`) {
		t.Fatalf("el cambio debe exponer fecha y hora: %s", w.Body)
	}

	w = llamar(h, "GET", "/api/escolar/apoderado", "", escolar)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"autorizado":false`) {
		t.Fatalf("el escolar ve la revocación: %d %s", w.Code, w.Body)
	}

	w = llamar(h, "POST", ruta, `{"autorizado":true}`, apoderado)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"estado":"autorizado"`) {
		t.Fatalf("autorizar: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "POST", ruta, `{}`, apoderado); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"autorizado"`) {
		t.Fatalf("falta el estado: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "POST", ruta, `{"autorizado":false}`, escolar); w.Code != http.StatusForbidden {
		t.Fatalf("el escolar no cambia su autorización: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "POST", ruta, `{"autorizado":false}`, tutor); w.Code != http.StatusForbidden {
		t.Fatalf("el tutor no cambia autorizaciones: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "POST", ruta, `{"autorizado":false}`, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "POST", "/api/apoderado/escolares/00000000-0000-0000-0000-000000000000/autorizacion", `{"autorizado":false}`, apoderado); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"sin_apoderado"`) {
		t.Fatalf("escolar sin vínculo: %d %s", w.Code, w.Body)
	}

	rutaHistorial := "/api/apoderado/escolares/" + escolarID + "/autorizaciones"
	w = llamar(h, "GET", rutaHistorial, "", apoderado)
	if w.Code != http.StatusOK {
		t.Fatalf("historial: %d %s", w.Code, w.Body)
	}
	var historial struct {
		Autorizaciones []struct {
			Estado     string `json:"estado"`
			Autorizado bool   `json:"autorizado"`
			CreadaEn   string `json:"creada_en"`
		} `json:"autorizaciones"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &historial); err != nil {
		t.Fatal(err)
	}
	if len(historial.Autorizaciones) != 3 {
		t.Fatalf("esperaba aceptación, revocación y autorización: %s", w.Body)
	}
	for i, estado := range []string{"autorizado", "no_autorizado", "autorizado"} {
		if historial.Autorizaciones[i].Estado != estado || historial.Autorizaciones[i].CreadaEn == "" {
			t.Fatalf("historial[%d]=%+v", i, historial.Autorizaciones[i])
		}
		if historial.Autorizaciones[i].Autorizado != (estado == "autorizado") {
			t.Fatalf("autorizado incoherente en historial[%d]: %+v", i, historial.Autorizaciones[i])
		}
	}
	if w = llamar(h, "GET", rutaHistorial, "", escolar); w.Code != http.StatusForbidden {
		t.Fatalf("el escolar no ve el historial del apoderado: %d %s", w.Code, w.Body)
	}
	if w = llamar(h, "GET", rutaHistorial, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("historial sin sesión: %d %s", w.Code, w.Body)
	}
}

func TestVinculacionRechazaRolesYFaltasDeSesion(t *testing.T) {
	h, _ := servidor(t)
	apoderado := ingresar(t, h, "apoderado.prueba@example.test")
	escolar := ingresar(t, h, "escolar.prueba@example.test")

	if w := llamar(h, "POST", "/api/apoderado/vinculaciones", `{"email_escolar":"escolar.prueba@example.test"}`, escolar); w.Code != http.StatusForbidden {
		t.Fatalf("escolar no solicita: %d %s", w.Code, w.Body)
	}
	if w := llamar(h, "POST", "/api/escolar/vinculaciones/aceptar", `{"token":"x"}`, apoderado); w.Code != http.StatusForbidden {
		t.Fatalf("apoderado no acepta: %d %s", w.Code, w.Body)
	}
	if w := llamar(h, "GET", "/api/apoderado/vinculaciones", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d %s", w.Code, w.Body)
	}
	if w := llamar(h, "POST", "/api/apoderado/vinculaciones", `{"email_escolar":"tutor.prueba@uni.edu.pe"}`, apoderado); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("destino que no es escolar: %d %s", w.Code, w.Body)
	}
}
