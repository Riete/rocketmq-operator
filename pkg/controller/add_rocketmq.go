package controller

import (
	"github.com/riete/rocketmq-operator/pkg/controller/rocketmq"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, rocketmq.Add)
}
