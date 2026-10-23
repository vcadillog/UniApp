package reserva

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ctrlAcceso "uniapp/internal/controllers/acceso"
	infra "uniapp/internal/infra"
	modeloAcceso "uniapp/internal/models/acceso"
	modelo "uniapp/internal/models/reserva"
)

const (
	origen     = "http://localhost:5173"
	clave      = "Clave-Segura-2026!"
	cookieName = "uniapp_sesion"
	cuerpoOK   = `{"slot_id":"slot-i04-01","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to de secundaria"}`
)

// almacenPrueba guarda los adjuntos en memoria; el modelo solo conoce el puerto Almacen.
type almacenPrueba struct {
	mu       sync.Mutex
	archivos map[string][]byte
}

func (a *almacenPrueba) Guardar(_ context.Context, clave string, contenido []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.archivos[clave] = append([]byte(nil), contenido...)
	return nil
}

func (a *almacenPrueba) Leer(_ context.Context, clave string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	contenido, existe := a.archivos[clave]
	if !existe {
		return nil, modelo.ErrAlmacen
	}
	return contenido, nil
}

func (a *almacenPrueba) Eliminar(_ context.Context, clave string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.archivos, clave)
	return nil
}

type entorno struct {
	manejador   http.Handler
	escolar     *http.Cookie
	tutor       *http.Cookie
	coordinador *http.Cookie
}

type autorizacionFija bool

func (a autorizacionFija) AutorizacionVigente(context.Context, string) (bool, error) {
	return bool(a), nil
}

type autorizacionVariable struct{ permitido bool }

func (a *autorizacionVariable) AutorizacionVigente(context.Context, string) (bool, error) {
	return a.permitido, nil
}

func nuevoEntorno(t *testing.T) *entorno {
	return nuevoEntornoCon(t, modelo.AnalizadorBasico{})
}

func nuevoEntornoCon(t *testing.T, analizador modelo.Analizador) *entorno {
	t.Helper()
	return nuevoEntornoConAutorizacion(t, autorizacionFija(true), analizador)
}

func nuevoEntornoConAutorizacion(t *testing.T, autorizacion modelo.PuertoAutorizacion, analizador modelo.Analizador) *entorno {
	t.Helper()
	hasher := infra.PBKDF2Hasher{Iteraciones: 1000}
	servicioAcceso := modeloAcceso.NuevoServicio(modeloAcceso.Dependencias{
		Repositorio: modeloAcceso.NuevoRepositorioMemoria(),
		Fabrica:     modeloAcceso.NuevaFabrica(hasher, "uni.edu.pe"),
		Hasher:      hasher,
		Bus:         infra.NuevoBus(),
	})
	if err := modeloAcceso.Sembrar(context.Background(), servicioAcceso, modeloAcceso.OpcionesSiembra{DominioTutor: "uni.edu.pe", Clave: clave}); err != nil {
		t.Fatal(err)
	}
	controladorAcceso := ctrlAcceso.Nuevo(servicioAcceso, false)

	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	almacen := &almacenPrueba{archivos: map[string][]byte{}}
	servicioReserva := modelo.NuevoServicio(modelo.Dependencias{
		Repositorio:  modelo.NuevoRepositorioMemoria(),
		Oferta:       modelo.NuevaOfertaSintetica(ahora),
		Autorizacion: autorizacion,
		Almacen:      almacen,
		Analizador:   analizador,
	})
	servicioReserva.FijarReloj(func() time.Time { return ahora })

	manejadorReserva := Nuevo(servicioReserva, controladorAcceso.Autenticar()).Rutas()
	raiz := http.NewServeMux()
	// Igual que cmd/server: el prefijo exacto evita el redirect 301 de POST /api/reserva.
	raiz.Handle("/api/reserva", manejadorReserva)
	raiz.Handle("/api/reserva/", manejadorReserva)
	raiz.Handle("/", controladorAcceso.Rutas())
	manejador := ctrlAcceso.Encadenar(raiz, ctrlAcceso.OrigenPermitido(origen))

	return &entorno{
		manejador:   manejador,
		escolar:     ingresar(t, manejador, "escolar.prueba@example.test"),
		tutor:       ingresar(t, manejador, "tutor.prueba@uni.edu.pe"),
		coordinador: ingresar(t, manejador, "coord.prueba@example.test"),
	}
}

func llamar(manejador http.Handler, metodo, ruta, cuerpo string, cookie *http.Cookie) *httptest.ResponseRecorder {
	peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		peticion.AddCookie(cookie)
	}
	grabador := httptest.NewRecorder()
	manejador.ServeHTTP(grabador, peticion)
	return grabador
}

func llamarConClave(manejador http.Handler, metodo, ruta, cuerpo, idempotencia string, cookie *http.Cookie) *httptest.ResponseRecorder {
	peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Idempotency-Key", idempotencia)
	if cookie != nil {
		peticion.AddCookie(cookie)
	}
	grabador := httptest.NewRecorder()
	manejador.ServeHTTP(grabador, peticion)
	return grabador
}

func subirArchivo(manejador http.Handler, reservaID, nombre string, contenido []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
	var cuerpo bytes.Buffer
	escritor := multipart.NewWriter(&cuerpo)
	parte, err := escritor.CreateFormFile("archivo", nombre)
	if err != nil {
		panic(err)
	}
	if _, err := parte.Write(contenido); err != nil {
		panic(err)
	}
	if err := escritor.Close(); err != nil {
		panic(err)
	}
	peticion := httptest.NewRequest("POST", "/api/reserva/"+reservaID+"/adjuntos", &cuerpo)
	peticion.Header.Set("Content-Type", escritor.FormDataContentType())
	if cookie != nil {
		peticion.AddCookie(cookie)
	}
	grabador := httptest.NewRecorder()
	manejador.ServeHTTP(grabador, peticion)
	return grabador
}

func ingresar(t *testing.T, manejador http.Handler, email string) *http.Cookie {
	t.Helper()
	respuesta := llamar(manejador, "POST", "/api/auth/ingreso", `{"email":"`+email+`","clave":"`+clave+`"}`, nil)
	if respuesta.Code != http.StatusOK {
		t.Fatalf("ingreso %s: %d %s", email, respuesta.Code, respuesta.Body)
	}
	for _, cookie := range respuesta.Result().Cookies() {
		if cookie.Name == cookieName {
			return cookie
		}
	}
	t.Fatal("sin cookie de sesión")
	return nil
}

func decodificar(t *testing.T, respuesta *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var cuerpo map[string]any
	if err := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("JSON inválido (%d): %s", respuesta.Code, respuesta.Body)
	}
	return cuerpo
}

func crearReserva(t *testing.T, manejador http.Handler, cookie *http.Cookie) map[string]any {
	t.Helper()
	respuesta := llamar(manejador, "POST", "/api/reserva", cuerpoOK, cookie)
	if respuesta.Code != http.StatusCreated {
		t.Fatalf("crear: %d %s", respuesta.Code, respuesta.Body)
	}
	return decodificar(t, respuesta)
}

func pdfDePrueba() []byte {
	return []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
}

func TestReservaBaseDeExtremoAExtremo(t *testing.T) {
	entorno := nuevoEntorno(t)

	listado := llamar(entorno.manejador, "GET", "/api/reserva/slots", "", entorno.escolar)
	if listado.Code != http.StatusOK {
		t.Fatalf("slots: %d %s", listado.Code, listado.Body)
	}
	var slots struct {
		Slots []struct {
			ID         string `json:"id"`
			Disponible bool   `json:"disponible"`
		} `json:"slots"`
	}
	if err := json.Unmarshal(listado.Body.Bytes(), &slots); err != nil || len(slots.Slots) == 0 {
		t.Fatalf("lista de horarios: %v %s", err, listado.Body)
	}

	reserva := crearReserva(t, entorno.manejador, entorno.escolar)
	if reserva["estado"] != "confirmada" || reserva["curso"] != "Álgebra" {
		t.Fatalf("reserva confirmada con el horario elegido: %v", reserva)
	}
	if reserva["inicio_lima"] == "" || reserva["tema"] == "" {
		t.Fatalf("la respuesta presenta la hora de Lima: %v", reserva)
	}
	if adjuntos, ok := reserva["adjuntos"].([]any); !ok || len(adjuntos) != 0 {
		t.Fatalf("la reserva nace sin adjuntos: %v", reserva["adjuntos"])
	}

	consultada := llamar(entorno.manejador, "GET", "/api/reserva/"+reserva["id"].(string), "", entorno.escolar)
	if consultada.Code != http.StatusOK {
		t.Fatalf("consultar: %d %s", consultada.Code, consultada.Body)
	}
}

func TestRevocarAutorizacionNoAfectaReservasConfirmadas(t *testing.T) {
	autorizacion := &autorizacionVariable{permitido: true}
	entorno := nuevoEntornoConAutorizacion(t, autorizacion, modelo.AnalizadorBasico{})

	creada := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, entorno.escolar)
	if creada.Code != http.StatusCreated {
		t.Fatalf("reserva inicial: %d %s", creada.Code, creada.Body)
	}
	id := decodificar(t, creada)["id"].(string)

	autorizacion.permitido = false
	nueva := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, entorno.escolar)
	if nueva.Code != http.StatusForbidden || decodificar(t, nueva)["codigo"] != "sin_autorizacion" {
		t.Fatalf("sin autorización no se reserva: %d %s", nueva.Code, nueva.Body)
	}

	existente := llamar(entorno.manejador, "GET", "/api/reserva/"+id, "", entorno.escolar)
	if existente.Code != http.StatusOK {
		t.Fatalf("la reserva ya confirmada sigue vigente: %d %s", existente.Code, existente.Body)
	}
}

func TestReservaRequiereSesionDeEscolar(t *testing.T) {
	entorno := nuevoEntorno(t)
	if respuesta := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, nil); respuesta.Code != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", respuesta.Code)
	}
	respuesta := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, entorno.tutor)
	if respuesta.Code != http.StatusForbidden || decodificar(t, respuesta)["codigo"] != "solo_escolar" {
		t.Fatalf("un tutor no reserva: %d %s", respuesta.Code, respuesta.Body)
	}
	if respuesta := llamar(entorno.manejador, "GET", "/api/reserva/slots", "", nil); respuesta.Code != http.StatusUnauthorized {
		t.Fatalf("slots sin sesión: %d", respuesta.Code)
	}
}

func TestReservaRechazaDatosYReglas(t *testing.T) {
	entorno := nuevoEntorno(t)
	casos := []struct {
		nombre string
		cuerpo string
		estado int
		codigo string
		campo  string
	}{
		{"tema corto", `{"slot_id":"slot-i04-01","tema":"corto","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to"}`, http.StatusUnprocessableEntity, "validacion", "tema"},
		{"sin grado", `{"slot_id":"slot-i04-01","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":""}`, http.StatusUnprocessableEntity, "validacion", "grado"},
		{"adjuntos en el cuerpo", `{"slot_id":"slot-i04-01","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to","adjuntos":[]}`, http.StatusBadRequest, "solicitud_invalida", ""},
		{"menos de 48 horas", `{"slot_id":"slot-i04-cercano","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to"}`, http.StatusUnprocessableEntity, "anticipacion_insuficiente", "slot_id"},
		{"fuera del horizonte", `{"slot_id":"slot-i04-lejano","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to"}`, http.StatusUnprocessableEntity, "horizonte_excedido", "slot_id"},
		{"slot inexistente", `{"slot_id":"slot-fantasma","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to"}`, http.StatusNotFound, "slot_inexistente", ""},
		{"clave de idempotencia inválida", `{"slot_id":"slot-i04-01","tema":"Repaso de factorización","objetivo":"Resolver ejercicios de factorización de trinomios sin ayuda.","grado":"5to"}`, http.StatusUnprocessableEntity, "validacion", "idempotency_key"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			var respuesta *httptest.ResponseRecorder
			if caso.campo == "idempotency_key" {
				respuesta = llamarConClave(entorno.manejador, "POST", "/api/reserva", caso.cuerpo, "corta", entorno.escolar)
			} else {
				respuesta = llamar(entorno.manejador, "POST", "/api/reserva", caso.cuerpo, entorno.escolar)
			}
			if respuesta.Code != caso.estado {
				t.Fatalf("estado %d, esperado %d: %s", respuesta.Code, caso.estado, respuesta.Body)
			}
			cuerpo := decodificar(t, respuesta)
			if cuerpo["codigo"] != caso.codigo {
				t.Fatalf("código %v, esperado %s", cuerpo["codigo"], caso.codigo)
			}
			if caso.campo != "" {
				campos, _ := cuerpo["campos"].(map[string]any)
				if campos[caso.campo] == nil {
					t.Fatalf("falta el campo %q en %v", caso.campo, cuerpo["campos"])
				}
			}
		})
	}
}

func TestReservaNoDuplicaSlotYSoloLaVeSuDueno(t *testing.T) {
	entorno := nuevoEntorno(t)
	primera := crearReserva(t, entorno.manejador, entorno.escolar)
	repetida := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, entorno.escolar)
	if repetida.Code != http.StatusConflict || decodificar(t, repetida)["codigo"] != "slot_ocupado" {
		t.Fatalf("segunda reserva del mismo slot: %d %s", repetida.Code, repetida.Body)
	}
	id := primera["id"].(string)
	if respuesta := llamar(entorno.manejador, "GET", "/api/reserva/"+id, "", entorno.tutor); respuesta.Code != http.StatusForbidden {
		t.Fatalf("el tutor no ve la reserva ajena: %d", respuesta.Code)
	}
	if respuesta := llamar(entorno.manejador, "GET", "/api/reserva/00000000-0000-0000-0000-000000000000", "", entorno.escolar); respuesta.Code != http.StatusNotFound {
		t.Fatalf("reserva inexistente: %d", respuesta.Code)
	}
}

func TestReintentoIdempotenteDelFormulario(t *testing.T) {
	entorno := nuevoEntorno(t)
	claveFormulario := "formulario-2026-10-20-001"
	primera := llamarConClave(entorno.manejador, "POST", "/api/reserva", cuerpoOK, claveFormulario, entorno.escolar)
	if primera.Code != http.StatusCreated || primera.Header().Get("Idempotent-Replayed") != "false" {
		t.Fatalf("primera reserva: %d %s", primera.Code, primera.Body)
	}
	id := decodificar(t, primera)["id"].(string)

	reintento := llamarConClave(entorno.manejador, "POST", "/api/reserva", cuerpoOK, claveFormulario, entorno.escolar)
	if reintento.Code != http.StatusOK || reintento.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("el reintento debe ser 200: %d %s", reintento.Code, reintento.Body)
	}
	if decodificar(t, reintento)["id"] != id {
		t.Fatal("el reintento debe devolver la misma reserva")
	}

	otroCuerpo := strings.Replace(cuerpoOK, "Repaso de factorización", "Otro tema totalmente distinto", 1)
	if respuesta := llamarConClave(entorno.manejador, "POST", "/api/reserva", otroCuerpo, claveFormulario, entorno.escolar); respuesta.Code != http.StatusConflict || decodificar(t, respuesta)["codigo"] != "idempotencia_conflicto" {
		t.Fatalf("misma clave con otros datos: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestCuotaDeReservasFuturasEnLaAPI(t *testing.T) {
	entorno := nuevoEntorno(t)
	if respuesta := llamar(entorno.manejador, "POST", "/api/reserva", cuerpoOK, entorno.escolar); respuesta.Code != http.StatusCreated {
		t.Fatalf("primera: %d %s", respuesta.Code, respuesta.Body)
	}
	segunda := strings.Replace(cuerpoOK, "slot-i04-01", "slot-i04-02", 1)
	if respuesta := llamar(entorno.manejador, "POST", "/api/reserva", segunda, entorno.escolar); respuesta.Code != http.StatusCreated {
		t.Fatalf("segunda: %d %s", respuesta.Code, respuesta.Body)
	}
	tercera := strings.Replace(cuerpoOK, "slot-i04-01", "slot-i04-03", 1)
	respuesta := llamar(entorno.manejador, "POST", "/api/reserva", tercera, entorno.escolar)
	if respuesta.Code != http.StatusConflict || decodificar(t, respuesta)["codigo"] != "cuota_escolar" {
		t.Fatalf("tercera reserva futura: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestAdjuntoSeSubeYSeDescargaConPermisos(t *testing.T) {
	entorno := nuevoEntorno(t)
	reserva := crearReserva(t, entorno.manejador, entorno.escolar)
	id := reserva["id"].(string)

	subida := subirArchivo(entorno.manejador, id, "practica.pdf", pdfDePrueba(), entorno.escolar)
	if subida.Code != http.StatusCreated {
		t.Fatalf("subir adjunto: %d %s", subida.Code, subida.Body)
	}
	adjunto := decodificar(t, subida)
	if adjunto["estado"] != "validado" || adjunto["descargable"] != true {
		t.Fatalf("adjunto validado: %v", adjunto)
	}
	if _, expuesta := adjunto["clave_almacenamiento"]; expuesta {
		t.Fatal("la clave de almacenamiento no debe exponerse")
	}

	listado := llamar(entorno.manejador, "GET", "/api/reserva/"+id+"/adjuntos", "", entorno.escolar)
	if listado.Code != http.StatusOK {
		t.Fatalf("listar adjuntos: %d %s", listado.Code, listado.Body)
	}
	var lista struct {
		Adjuntos []map[string]any `json:"adjuntos"`
	}
	if err := json.Unmarshal(listado.Body.Bytes(), &lista); err != nil || len(lista.Adjuntos) != 1 {
		t.Fatalf("lista de adjuntos: %v %s", err, listado.Body)
	}

	descarga := llamar(entorno.manejador, "GET", "/api/reserva/"+id+"/adjuntos/"+adjunto["id"].(string), "", entorno.escolar)
	if descarga.Code != http.StatusOK || !bytes.Equal(descarga.Body.Bytes(), pdfDePrueba()) {
		t.Fatalf("descarga del dueño: %d %s", descarga.Code, descarga.Body)
	}
	if descarga.Header().Get("Content-Disposition") == "" || descarga.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("el enlace privado debe forzar descarga y no cachear")
	}

	// La oferta sintética usa un tutor ficticio sin cuenta; coordinación sí es un
	// participante autorizado y el caso del tutor real se cubre en el modelo.
	if respuesta := llamar(entorno.manejador, "GET", "/api/reserva/"+id+"/adjuntos/"+adjunto["id"].(string), "", entorno.coordinador); respuesta.Code != http.StatusOK {
		t.Fatalf("coordinación supervisa y descarga: %d %s", respuesta.Code, respuesta.Body)
	}
	if respuesta := llamar(entorno.manejador, "GET", "/api/reserva/"+id+"/adjuntos/"+adjunto["id"].(string), "", entorno.tutor); respuesta.Code != http.StatusForbidden {
		t.Fatalf("un tutor ajeno a la reserva no descarga: %d %s", respuesta.Code, respuesta.Body)
	}

	if respuesta := subirArchivo(entorno.manejador, id, "ajeno.pdf", pdfDePrueba(), entorno.tutor); respuesta.Code != http.StatusForbidden {
		t.Fatalf("el tutor tampoco sube adjuntos: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestAdjuntoEnCuarentenaYLimite(t *testing.T) {
	entorno := nuevoEntornoCon(t, modelo.AnalizadorNoDisponible{})
	reserva := crearReserva(t, entorno.manejador, entorno.escolar)
	id := reserva["id"].(string)

	subida := subirArchivo(entorno.manejador, id, "practica.pdf", pdfDePrueba(), entorno.escolar)
	if subida.Code != http.StatusCreated {
		t.Fatalf("subir sin análisis: %d %s", subida.Code, subida.Body)
	}
	adjunto := decodificar(t, subida)
	if adjunto["estado"] != "en_cuarentena" || adjunto["descargable"] != false {
		t.Fatalf("sin análisis queda en cuarentena: %v", adjunto)
	}
	descarga := llamar(entorno.manejador, "GET", "/api/reserva/"+id+"/adjuntos/"+adjunto["id"].(string), "", entorno.escolar)
	if descarga.Code != http.StatusConflict || decodificar(t, descarga)["codigo"] != "adjunto_en_cuarentena" {
		t.Fatalf("no se descarga en cuarentena: %d %s", descarga.Code, descarga.Body)
	}

	for i := 1; i < modelo.MaxAdjuntos; i++ {
		if respuesta := subirArchivo(entorno.manejador, id, "practica.pdf", pdfDePrueba(), entorno.escolar); respuesta.Code != http.StatusCreated {
			t.Fatalf("adjunto %d: %d %s", i, respuesta.Code, respuesta.Body)
		}
	}
	sexto := subirArchivo(entorno.manejador, id, "practica.pdf", pdfDePrueba(), entorno.escolar)
	if sexto.Code != http.StatusConflict || decodificar(t, sexto)["codigo"] != "adjuntos_llenos" {
		t.Fatalf("sexto adjunto: %d %s", sexto.Code, sexto.Body)
	}
}

func TestAdjuntoInvalidoYMuyGrande(t *testing.T) {
	entorno := nuevoEntorno(t)
	reserva := crearReserva(t, entorno.manejador, entorno.escolar)
	id := reserva["id"].(string)

	html := subirArchivo(entorno.manejador, id, "pagina.html", []byte("<html><body>hola</body></html>"), entorno.escolar)
	if html.Code != http.StatusUnprocessableEntity || decodificar(t, html)["codigo"] != "adjunto_invalido" {
		t.Fatalf("tipo no permitido: %d %s", html.Code, html.Body)
	}
	enorme := make([]byte, modelo.AdjuntoMaximoBytes+1)
	copy(enorme, pdfDePrueba())
	grande := subirArchivo(entorno.manejador, id, "enorme.pdf", enorme, entorno.escolar)
	if grande.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("archivo demasiado grande: %d %s", grande.Code, grande.Body)
	}
}
