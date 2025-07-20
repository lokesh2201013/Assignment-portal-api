package Error

import(
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
)

func ReturnError(ServiceName string,Err error,Message string)(models.Error){

	return models.Error{
		ServiceName: ServiceName,
		Message:  Message,
		Description: Err.Error(),}
}