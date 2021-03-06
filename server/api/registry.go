package api

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/harbor-incubator/octopus/server/core"
	"github.com/harbor-incubator/octopus/server/model"
	"github.com/harbor-incubator/octopus/server/util"
)

func GetRegistry(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	registry, err := core.DefaultRegMgr.Get(id)
	if err != nil {
		handleNotFound(rw)
		return
	}

	if err := writeJSON(rw, registry); err != nil {
		handleInternalServerError(rw, err)
		return
	}

}

func ListRegistry(rw http.ResponseWriter, r *http.Request) {
	registries, err := core.DefaultRegMgr.List()
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if err = writeJSON(rw, registries); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func CreateRegistry(rw http.ResponseWriter, r *http.Request) {
	registry := &model.Registry{}
	if err := readJSON(r, registry); err != nil {
		log.Printf("%v \n", err)
		handleBadRequest(rw)
		return
	}
	id, err := core.DefaultRegMgr.Create(registry)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if err = writeJSON(rw, map[string]string{"id": id}); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func UpdateRegistry(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := core.DefaultRegMgr.Get(id); err != nil {
		handleNotFound(rw)
		return
	}

	registry := &model.Registry{}
	if err := readJSON(r, registry); err != nil {
		log.Printf("%v \n", err)
		handleBadRequest(rw)
		return
	}
	if err := core.DefaultRegMgr.Update(registry); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func DeleteRegistry(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// TODO: check if the registry is referenced by topylogy graph
	if err := core.DefaultRegMgr.Delete(id); err != nil {
		if err == core.ErrRegNotFound {
			handleNotFound(rw)
			return
		}
		handleInternalServerError(rw, err)
		return
	}
}

func PingRegistry(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := core.DefaultRegMgr.Ping(id); err != nil {
		if err == core.ErrRegNotFound {
			handleNotFound(rw)
			return
		}
		log.Printf("%v \n", err)
		handleBadRequest(rw)
		return
	}
}

func ListProjects(rw http.ResponseWriter, r *http.Request) {
	registryID := mux.Vars(r)["id"]
	registry, err := core.DefaultRegMgr.Get(registryID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if registry == nil {
		log.Printf("registry %s not found \n", registryID)
		handleNotFound(rw)
		return
	}
	client := util.New(registry.URL, registry.Username, registry.Password, registry.Insecure)
	projects, err := client.GetProjects()
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}

	if err = writeJSON(rw, projects); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}
