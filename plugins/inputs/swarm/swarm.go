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
	acc.AddFields("swarm", map[string]interface{}{"n_nodes": len(nodes)}, nil, now) //map[string]string{"unit": "bytes"}, now)
	for _, node := range nodes {
		nodeMap[node.ID] = node.Description.Hostname
		acc.AddFields("swarm_node", map[string]interface{}{
			"id":         node.ID,
			"hostname":   node.Description.Hostname,
			"cpu_shares": node.Description.Resources.NanoCPUs,
			"memory":     node.Description.Resources.MemoryBytes,
		}, nil, now) //map[string]string{"unit": "bytes"}, now)
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

	for _, task := range tasks {
		acc.AddFields("swarm_task", map[string]interface{}{
			"id":           task.ID,
			"service_id":   task.ServiceID,
			"service_name": serviceMap[task.ServiceID],
			"slot":         task.Slot,
			"node_id":      task.NodeID,
			"node_name":    nodeMap[task.NodeID],
			"image":        task.Spec.ContainerSpec.Image,
		}, nil, now)
	}

	return nil
}

func init() {
	inputs.Add("swarm", func() telegraf.Input {
		return &Swarm{
			Timeout: internal.Duration{Duration: time.Second * 5},
		}
	})

}
