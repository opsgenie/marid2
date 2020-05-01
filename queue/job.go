package queue

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/opsgenie/oec/runbook"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	JobInitial = iota
	JobExecuting
	JobFinished
	JobError
)

type Job interface {
	Id() string
	Execute() error
}

type SqsJob struct {
	queueProvider QueueProvider
	queueMessage  QueueMessage

	ownerId string
	apiKey  string
	baseUrl string

	state        int32
	executeMutex *sync.Mutex
}

func NewSqsJob(queueMessage QueueMessage, queueProvider QueueProvider, apiKey, baseUrl, ownerId string) Job {
	return &SqsJob{
		queueProvider: queueProvider,
		queueMessage:  queueMessage,
		executeMutex:  &sync.Mutex{},
		apiKey:        apiKey,
		baseUrl:       baseUrl,
		ownerId:       ownerId,
		state:         JobInitial,
	}
}

func (j *SqsJob) Id() string {
	return *j.queueMessage.Message().MessageId
}

func (j *SqsJob) sqsMessage() *sqs.Message {
	return j.queueMessage.Message()
}

func (j *SqsJob) Execute() error {

	defer j.executeMutex.Unlock()
	j.executeMutex.Lock()

	if j.state != JobInitial {
		return errors.Errorf("Job[%s] is already executing or finished.", j.Id())
	}
	j.state = JobExecuting

	region := j.queueProvider.OECMetadata().Region()
	messageId := j.Id()

	err := j.queueProvider.DeleteMessage(j.sqsMessage())
	if err != nil {
		j.state = JobError
		return errors.Errorf("Message[%s] could not be deleted from the queue[%s]: %s", messageId, region, err)
	}

	logrus.Debugf("Message[%s] is deleted from the queue[%s].", messageId, region)

	messageAttr := j.sqsMessage().MessageAttributes

	if messageAttr == nil ||
		*messageAttr[ownerId].StringValue != j.ownerId {
		j.state = JobError
		return errors.Errorf("Message[%s] is invalid, will not be processed.", messageId)
	}

	result, err := j.queueMessage.Process()
	if err != nil {
		j.state = JobError
		return errors.Errorf("Message[%s] could not be processed: %s", messageId, err)
	}

	go func() {
		start := time.Now()

		err = runbook.SendResultToOpsGenieFunc(result, j.apiKey, j.baseUrl)
		if err != nil {
			logrus.Warnf("Could not send action result[%+v] of message[%s] to Opsgenie: %s", result, messageId, err)
		} else {
			took := time.Since(start)
			logrus.Debugf("Successfully sent result of message[%s] to OpsGenie and it took %f seconds.", messageId, took.Seconds())
		}
	}()

	j.state = JobFinished
	return nil
}
