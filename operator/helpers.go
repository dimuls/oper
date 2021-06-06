package operator

import (
	"oper/entity"

	"github.com/sirupsen/logrus"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func HandleCanceledOperationProcessing(op *entity.OperationProcessing) {
	logrus.WithField("operation_processing_id", entity.UUIDToStr(op.Id)).Info("handling canceled operation processing")
	cancelFrom := op.Steps[op.CurrentStep].StartCancelFromStep
	op.Steps = append(op.Steps[:op.CurrentStep], op.CancelSteps[cancelFrom:]...)
	op.Canceled = true
}

func HandleErrorOperationProcessing(op *entity.OperationProcessing, err error) {
	logrus.WithField("operation_processing_id", entity.UUIDToStr(op.Id)).Info("handling error operation processing")
	op.Steps[op.CurrentStep].Error = true
	op.Steps[op.CurrentStep].ErrorMessage = err.Error()
	op.Steps[op.CurrentStep].DoneAt = timestamppb.Now()
	op.Canceled = true
	op.Error = true
	cancelFrom := op.Steps[op.CurrentStep].StartCancelFromStep
	op.Steps = append(op.Steps[:op.CurrentStep+1], op.CancelSteps[cancelFrom:]...)
	op.CurrentStep += 2
}
