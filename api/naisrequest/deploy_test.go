package naisrequest

import (
 	"encoding/json"
	"testing"
     "github.com/nais/naisd/api/naisrequest"
     "github.com/stretchr/testify/assert"
 )

 func TestStringMethodInDeployNaisRequestShouldHidePasswordAndUsername(t *testing.T) {
	  deployRequest := naisrequest.Deploy{
			FasitUsername: "username" ,
			FasitPassword: "password",
			Namespace:     "app",
		}
		
		jsonValue,err := json.Marshal(deployRequest.String())
		if err != nil{
			panic(err)
		}
		assert.Contains(t, string(jsonValue), "***")
		assert.Contains(t, string(jsonValue), "fasitPassword")
 }


 func TestStringMethodInDeployNaisRequestShouldNotHidePasswordAndUsername(t *testing.T) {
	  deployRequest := naisrequest.Deploy{
			FasitUsername: "username" ,
			FasitPassword: "password",
			Namespace:     "app",
		}
		
		jsonValue,err := json.Marshal(deployRequest)
		if err != nil{
			panic(err)
		}
		assert.Contains(t, string(jsonValue), "password")
		assert.Contains(t, string(jsonValue), "fasitPassword")
 }



