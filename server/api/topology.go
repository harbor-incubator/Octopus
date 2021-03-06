package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/harbor-incubator/octopus/server/core"
	"github.com/harbor-incubator/octopus/server/model"
	"github.com/harbor-incubator/octopus/server/util"
	common_http "github.com/vmware/harbor/src/common/http"
)

var (
	ErrEdgeNotFound = errors.New("edge not found")
)

func GetTopology(rw http.ResponseWriter, r *http.Request) {
	topology, err := core.DefaultTopologyMgr.Get()
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if err = writeJSON(rw, topology); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func CreateNode(rw http.ResponseWriter, r *http.Request) {
	registry := &model.Registry{}
	if err := readJSON(r, registry); err != nil {
		log.Printf("%v \n", err)
		handleBadRequest(rw)
		return
	}
	exist, err := core.DefaultRegMgr.Exist(registry.ID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if !exist {
		handleNotFound(rw)
		return
	}

	exist, err = core.DefaultTopologyMgr.NodeExist(registry.ID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if exist {
		handleConflict(rw)
		return
	}
	if err := core.DefaultTopologyMgr.CreateNode(registry.ID); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func DeleteNode(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	exist, err := core.DefaultTopologyMgr.NodeExist(id)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if !exist {
		handleNotFound(rw)
		return
	}
	if err = core.DefaultTopologyMgr.DeleteNode(id); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func CreateEdge(rw http.ResponseWriter, r *http.Request) {
	edge := &edgeReq{}
	if err := readJSON(r, edge); err != nil {
		log.Printf("%v \n", err)
		handleBadRequest(rw)
		return
	}
	srcReg, err := core.DefaultRegMgr.Get(edge.SRCNodeID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if srcReg == nil {
		log.Printf("src node %s not found \n", edge.SRCNodeID)
		handleNotFound(rw)
		return
	}
	dstReg, err := core.DefaultRegMgr.Get(edge.DSTNodeID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if dstReg == nil {
		log.Printf("dst node %s not found \n", edge.DSTNodeID)
		handleNotFound(rw)
		return
	}
	client := util.New(srcReg.URL, srcReg.Username, srcReg.Password, srcReg.Insecure)
	exist, targetId, err := client.TargetExist(dstReg.URL, dstReg.Username, dstReg.Insecure)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	// if the endpoint doesn't exist on src registry, create it first
	if !exist {
		targetId, err = client.CreateTarget(dstReg)
		if err != nil {
			handleInternalServerError(rw, err)
			return
		}
	}

	policy := &util.Policy{
		Name:        fmt.Sprintf("policy-%d", time.Now().Unix()),
		Description: edge.Policy.Description,
		Projects: []*util.Project{
			&util.Project{
				ID: edge.Policy.ProjectID,
			},
		},
		Targets: []*util.Target{
			&util.Target{
				ID: targetId,
			},
		},
		Filters:           edge.Policy.Filters,
		Trigger:           edge.Policy.Trigger,
		ReplicateDeletion: edge.Policy.ReplicateDeletion,
	}
	policyID, err := client.CreatePolicy(policy)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	id, err := core.DefaultTopologyMgr.CreateEdge(
		&model.Edge{
			ID:        edge.ID,
			SRCNodeID: srcReg.ID,
			DSTNodeID: dstReg.ID,
			PolicyID:  policyID,
		})
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if err = writeJSON(rw, map[string]string{"id": id}); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func DeleteEdge(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	edge, err := core.DefaultTopologyMgr.GetEdge(id)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if edge == nil {
		log.Printf("edge %s not found \n", id)
		handleNotFound(rw)
		return
	}

	registry, err := core.DefaultRegMgr.Get(edge.SRCNodeID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if registry == nil {
		handleInternalServerError(rw, fmt.Errorf("registry %s not exist", edge.SRCNodeID))
		return
	}

	client := util.New(registry.URL, registry.Username, registry.Password, registry.Insecure)
	if err = client.DeletePolicy(edge.PolicyID); err != nil {
		if er, ok := err.(*common_http.Error); ok && er.Code != 404 {
			handleInternalServerError(rw, err)
			return
		}
	}

	if err = core.DefaultTopologyMgr.DeleteEdge(id); err != nil {
		handleInternalServerError(rw, err)
		return
	}
}

func GetEdgeStatus(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	jobs, err := getEdgeStatus(id)
	if err != nil {
		if err == ErrEdgeNotFound {
			log.Printf("edge %s not found \n", id)
			handleNotFound(rw)
			return
		}
		handleInternalServerError(rw, err)
		return
	}

	if err = writeJSON(rw, jobs); err != nil {
		handleInternalServerError(rw, err)
	}
}

func GetEdgePolicy(rw http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	edge, err := core.DefaultTopologyMgr.GetEdge(id)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if edge == nil {
		log.Printf("edge %s not found \n", id)
		handleNotFound(rw)
		return
	}

	registry, err := core.DefaultRegMgr.Get(edge.SRCNodeID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if registry == nil {
		handleInternalServerError(rw, fmt.Errorf("registry %s not exist", edge.SRCNodeID))
		return
	}
	client := util.New(registry.URL, registry.Username, registry.Password, registry.Insecure)
	policy, err := client.GetPolicy(edge.PolicyID)
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	res := &edgeReq{
		ID:        edge.ID,
		SRCNodeID: edge.SRCNodeID,
		DSTNodeID: edge.DSTNodeID,
		Policy: &policyReq{
			Description:       policy.Description,
			ProjectID:         policy.Projects[0].ID,
			Filters:           policy.Filters,
			Trigger:           policy.Trigger,
			ReplicateDeletion: policy.ReplicateDeletion,
		},
	}
	if err = writeJSON(rw, res); err != nil {
		handleInternalServerError(rw, err)
	}
}

func GetTopologyStatus(rw http.ResponseWriter, r *http.Request) {
	status := map[string][]*model.Job{}
	topology, err := core.DefaultTopologyMgr.Get()
	if err != nil {
		handleInternalServerError(rw, err)
		return
	}
	if topology != nil {
		for _, edge := range topology.Edges {
			if edge == nil {
				continue
			}
			jobs, err := getEdgeStatus(edge.ID)
			if err != nil {
				handleInternalServerError(rw, err)
				return
			}
			status[edge.ID] = jobs
		}
	}

	if err = writeJSON(rw, status); err != nil {
		handleInternalServerError(rw, err)
	}
}

func getEdgeStatus(id string) ([]*model.Job, error) {
	edge, err := core.DefaultTopologyMgr.GetEdge(id)
	if err != nil {
		return nil, err
	}
	if edge == nil {
		return nil, ErrEdgeNotFound
	}

	registry, err := core.DefaultRegMgr.Get(edge.SRCNodeID)
	if err != nil {
		return nil, err
	}
	if registry == nil {
		return nil, fmt.Errorf("registry %s not exist", edge.SRCNodeID)
	}
	client := util.New(registry.URL, registry.Username, registry.Password, registry.Insecure)
	jobs, err := client.GetJobs(edge.PolicyID)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}
