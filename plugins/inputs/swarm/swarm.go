package swarm

import (
	//	"encoding/json"
	//	"fmt"
	//	"io"
	//	"log"
	//	"regexp"
	//	"strconv"
	//	"strings"
	//	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type Swarm struct {
	Endpoint string
	Timeout  internal.Duration
	client   *client.Client
}

// Description returns input description
func (s *Swarm) Description() string {
	return "Read metrics about Docker Swarm service"
}

// SampleConfig prints sampleConfig
func (s *Swarm) SampleConfig() string { return `` }

// Gather starts stats collection
func (s *Swarm) Gather(acc telegraf.Accumulator) error {

	if s.client == nil {
		var c *client.Client
		var err error
		defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
		if s.Endpoint == "ENV" {
			c, err = client.NewEnvClient()
			if err != nil {
				return err
			}
		} else if s.Endpoint == "" {
			c, err = client.NewClient("unix:///var/run/docker.sock", "", nil, defaultHeaders)
			if err != nil {
				return err
			}
		} else {
			c, err = client.NewClient(s.Endpoint, "", nil, defaultHeaders)
			if err != nil {
				return err
			}
		}
		s.client = c
	}
	/*

		// Get daemon info
		err := d.gatherInfo(acc)
		if err != nil {
			fmt.Println(err.Error())
		}

		// List containers
		opts := types.ContainerListOptions{}
		ctx, cancel := context.WithTimeout(context.Background(), d.Timeout.Duration)
		defer cancel()
		containers, err := d.client.ContainerList(ctx, opts)
		if err != nil {
			return err
		}

		// Get container data
		var wg sync.WaitGroup
		wg.Add(len(containers))
		for _, container := range containers {
			go func(c types.Container) {
				defer wg.Done()
				err := d.gatherContainer(c, acc)
				if err != nil {
					log.Printf("Error gathering container %s stats: %s\n",
						c.Names, err.Error())
				}
			}(container)
		}
		wg.Wait()
	*/
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout.Duration)
	defer cancel()

	nodes, err := s.client.NodeList(ctx, types.NodeListOptions{})
	nodeMap := make(map[string]string)
	if err != nil {
		return err
	}

	now := time.Now()

	for _, node := range nodes {
		nodeMap[node.ID] = node.Description.Hostname
		acc.AddFields("swarm_node", map[string]interface{}{
			"cpu_shares": node.Description.Resources.NanoCPUs,
			"memory":     node.Description.Resources.MemoryBytes,
		}, map[string]string{
			"node_id":       node.ID,
			"node_hostname": node.Description.Hostname,
		}, now) //map[string]string{"unit": "bytes"}, now)
	}

	services, err := s.client.ServiceList(ctx, types.ServiceListOptions{})
	serviceMap := make(map[string]string)
	for _, service := range services {
		serviceMap[service.ID] = service.Spec.Name
	}

	tasks, err := s.client.TaskList(ctx, types.TaskListOptions{})
	if err != nil {
		return err
	}

	taskByService := make(map[string][]swarm.Task)
	taskByStatus := make(map[string][]swarm.Task)
	// taskByStatusByService := make(map[string]map[string][]*swarm.Task)
	for _, task := range tasks {
		status := string(task.Status.State)
		grp, exist := taskByStatus[status]
		if exist == false {
			grp = []swarm.Task{}
		}
		grp = append(grp, task)
		taskByStatus[status] = grp

		service := serviceMap[task.ServiceID]
		sgrp, exist := taskByService[service]
		if exist == false {
			sgrp = []swarm.Task{}
		}
		sgrp = append(sgrp, task)
		taskByService[service] = sgrp

		acc.AddFields("swarm_task", map[string]interface{}{
			"n_tasks": 1,
			// "id":   task.ID,
			// "slot": task.Slot,
		}, map[string]string{
			"service_name": serviceMap[task.ServiceID],
			"node_name":    nodeMap[task.NodeID],
			"image":        task.Spec.ContainerSpec.Image,
			"status":       string(task.Status.State),
			"err":          task.Status.Err,
		}, now)
	}

	for s, tt := range taskByStatus {
		acc.AddFields("swarm_task_status", map[string]interface{}{
			"n_tasks": len(tt),
		}, map[string]string{
			"status": s,
		}, now)
	}
	/*
		acc.AddFields("swarm_service", map[string]interface{}{
		}, map[string]string{
			"n_tasks": service.
		}, now)
	*/

	acc.AddFields("swarm", map[string]interface{}{
		"n_nodes":    len(nodes),
		"n_services": len(services),
		"n_tasks":    len(tasks),
	}, nil, now)

	return nil
}

func init() {
	inputs.Add("swarm", func() telegraf.Input {
		return &Swarm{
			Timeout: internal.Duration{Duration: time.Second * 5},
		}
	})

}
