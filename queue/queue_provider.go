package queue

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"log"
	"sync"
	"net/http"
)

const tokenUrl = "https://app.opsgenie.com/v2/integrations/maridv2/credentials"
var httpNewRequest = http.NewRequest
var newSession = session.NewSession

type QueueProvider interface {
	ChangeMessageVisibility(message *sqs.Message, visibilityTimeout int64) error
	DeleteMessage(message *sqs.Message) error
	ReceiveMessage(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error)

	RefreshClient(assumeRoleResult *AssumeRoleResult) error
	GetQueueUrl() string
}

type MaridQueueProvider struct {

	region string
	queueName string
	queueUrl string

	client	*sqs.SQS
	rwMu	*sync.RWMutex

	ChangeMessageVisibilityMethod func(mqp *MaridQueueProvider, message *sqs.Message, visibilityTimeout int64) error
	DeleteMessageMethod           func(mqp *MaridQueueProvider, message *sqs.Message) error
	ReceiveMessageMethod          func(mqp *MaridQueueProvider, numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error)

	awsChangeMessageVisibilityMethod 	func(input *sqs.ChangeMessageVisibilityInput) (*sqs.ChangeMessageVisibilityOutput, error)
	awsDeleteMessageMethod 				func(input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
	awsReceiveMessageMethod       		func(input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)

	refreshClientMethod func(mqp *MaridQueueProvider, assumeRoleResult *AssumeRoleResult) error
	newConfigMethod     func(mqp *MaridQueueProvider, assumeRoleResult *AssumeRoleResult) *aws.Config
}

func NewQueueProvider(queueUrl string) QueueProvider {
	return &MaridQueueProvider{
		queueName:                     	uuid.New().String(),
		rwMu:                          	&sync.RWMutex{},
		queueUrl:					   	queueUrl,
		region:							"us-west-2",
		ChangeMessageVisibilityMethod: 	ChangeMessageVisibility,
		DeleteMessageMethod:           	DeleteMessage,
		ReceiveMessageMethod:          	ReceiveMessage,
		refreshClientMethod:           	RefreshClient,
		newConfigMethod:               	newConfig,
	}
}

/*func (mqp *MaridQueueProvider) getRegion() string {
	defer mqp.rwMu.RUnlock()
	mqp.rwMu.RLock()
	return mqp.token.getEndpoint()
}
func (mqp *MaridQueueProvider) GetQueueUrl() string {
	defer mqp.rwMu.RUnlock()
	mqp.rwMu.RLock()
	return mqp.token.GetQueueUrl()
}
func (mqp *MaridQueueProvider) getSuccessPeriod() time.Duration {
	defer mqp.rwMu.RUnlock()
	mqp.rwMu.RLock()
	return time.Duration(mqp.token.getSuccessRefreshPeriod()) * time.Second
}
func (mqp *MaridQueueProvider) getErrorPeriod() time.Duration {
	defer mqp.rwMu.RUnlock()
	mqp.rwMu.RLock()
	return time.Duration(mqp.token.getErrorRefreshPeriod()) * time.Second
}*/

func (mqp *MaridQueueProvider) GetQueueUrl() string {
	return mqp.queueUrl
}

func (mqp *MaridQueueProvider) ChangeMessageVisibility(message *sqs.Message, visibilityTimeout int64) error {
	return mqp.ChangeMessageVisibilityMethod(mqp, message, visibilityTimeout)
}

func (mqp *MaridQueueProvider) DeleteMessage(message *sqs.Message) error {
	return mqp.DeleteMessageMethod(mqp, message)
}

func (mqp *MaridQueueProvider) ReceiveMessage(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
	return mqp.ReceiveMessageMethod(mqp, numOfMessage, visibilityTimeout)
}

func (mqp *MaridQueueProvider) RefreshClient(assumeRoleResult *AssumeRoleResult) error {
	return mqp.refreshClientMethod(mqp, assumeRoleResult)
}

func (mqp *MaridQueueProvider) newConfig(assumeRoleResult *AssumeRoleResult) *aws.Config {
	return mqp.newConfigMethod(mqp, assumeRoleResult)
}

func ChangeMessageVisibility(mqp *MaridQueueProvider, message *sqs.Message, visibilityTimeout int64) error {

	request := &sqs.ChangeMessageVisibilityInput{
		ReceiptHandle:     message.ReceiptHandle,
		QueueUrl:          &mqp.queueUrl,
		VisibilityTimeout: &visibilityTimeout,
	}

	mqp.rwMu.RLock()
	resultVisibility, err := mqp.awsChangeMessageVisibilityMethod(request)
	mqp.rwMu.RUnlock()

	if err != nil {
		log.Printf("Change Message Visibility Error: %s", err)
		return err
	}

	log.Printf("Visibility Time Changed: %s", resultVisibility.String())
	return nil
}

func DeleteMessage(mqp *MaridQueueProvider, message *sqs.Message) error {

	request := &sqs.DeleteMessageInput{
		QueueUrl:      &mqp.queueUrl,
		ReceiptHandle: message.ReceiptHandle,
	}

	mqp.rwMu.RLock()
	resultDelete, err := mqp.awsDeleteMessageMethod(request)
	mqp.rwMu.RUnlock()

	if err != nil {
		log.Printf("Delete message error: %s", err)
		return err
	}
	log.Printf("Message deleted: %s", resultDelete.String())
	return nil
}

func ReceiveMessage(mqp *MaridQueueProvider, maxNumOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {

	request := &sqs.ReceiveMessageInput{ // todo check attributes
		AttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl:            &mqp.queueUrl,
		MaxNumberOfMessages: aws.Int64(maxNumOfMessage),
		VisibilityTimeout:   aws.Int64(visibilityTimeout),
		WaitTimeSeconds:     aws.Int64(0),
	}

	mqp.rwMu.RLock()
	result, err := mqp.awsReceiveMessageMethod(request)
	mqp.rwMu.RUnlock()

	if err != nil {
		log.Printf("Receive message error: %s", err)
		return nil, err
	}

	log.Printf("Received %d messages.", len(result.Messages))

	return result.Messages, nil
}

func RefreshClient(mqp *MaridQueueProvider, assumeRoleResult *AssumeRoleResult) error {

	config := mqp.newConfig(assumeRoleResult)
	sess, err := newSession(config)
	if err != nil {
		return err
	}

	mqp.rwMu.Lock()
	mqp.client = sqs.New(sess)
	mqp.awsChangeMessageVisibilityMethod = mqp.client.ChangeMessageVisibility
	mqp.awsDeleteMessageMethod = mqp.client.DeleteMessage
	mqp.awsReceiveMessageMethod = mqp.client.ReceiveMessage
	mqp.rwMu.Unlock()

	log.Printf("Client of queue provider[%s] has refreshed.", mqp.GetQueueUrl())

	return nil
}


func newConfig(mqp *MaridQueueProvider, assumeRoleResult *AssumeRoleResult) *aws.Config {

	ARRCredentials := assumeRoleResult.Credentials
	creds := credentials.NewStaticCredentials(ARRCredentials.AccessKeyId, ARRCredentials.SecretAccessKey, ARRCredentials.SessionToken)

	awsConfig := aws.NewConfig().WithRegion(mqp.region).WithCredentials(creds)

	return awsConfig
}
