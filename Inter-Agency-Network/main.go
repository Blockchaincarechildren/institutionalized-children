/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
        "os"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

// FilesPrivateChaincode example Chaincode implementation
type FilesPrivateChaincode struct {
}

type file struct {
	ObjectType string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Name       string `json:"name"`    //the fieldtags are needed to keep case from bouncing around
	IPFShash      string `json:"ipfshash"`
	Timestamp       int    `json:"timestamp"`
	Owner      string `json:"owner"`
}

type filePrivateDetails struct {
	ObjectType string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Name       string `json:"name"`    //the fieldtags are needed to keep case from bouncing around
	Folio      int    `json:"folio"`
}

// Init initializes chaincode
// ===========================
func (t *FilesPrivateChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

// Invoke - Our entry point for Invocations
// ========================================
func (t *FilesPrivateChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	// Handle different functions
	switch function {
	case "initFile":
		//create a new file
		return t.initFile(stub, args)
	case "readFile":
		//read a file
		return t.readFile(stub, args)
	case "readFilePrivateDetails":
		//read a file private details
		return t.readFilePrivateDetails(stub, args)
	case "transferFile":
		//change owner of a specific file
		return t.transferFile(stub, args)
	case "delete":
		//delete a file
		return t.delete(stub, args)
	case "getFilesByRange":
		//get files based on range query
		return t.getFilesByRange(stub, args)
	case "getFileHash":
		// get private data hash for collectionFiles
		return t.getFileHash(stub, args)
	case "getFilePrivateDetailsHash":
		// get private data hash for collectionFilePrivateDetails
		return t.getFilePrivateDetailsHash(stub, args)
	default:
		//error
		fmt.Println("invoke did not find func: " + function)
		return shim.Error("Received unknown function invocation")
	}
}

// ============================================================
// initFile - create a new file, store into chaincode state
// ============================================================
func (t *FilesPrivateChaincode) initFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error

	type fileTransientInput struct {
		Name  string `json:"name"` //the fieldtags are needed to keep case from bouncing around
		IPFShash string `json:"ipfshash"`
		Timestamp  int    `json:"size"`
		Owner string `json:"owner"`
		Folio int    `json:"folio"`
	}

	// ==== Input sanitation ====
	fmt.Println("- start init file")

	if len(args) != 0 {
		return shim.Error("Incorrect number of arguments. Private file data must be passed in transient map.")
	}

	transMap, err := stub.GetTransient()
	if err != nil {
		return shim.Error("Error getting transient: " + err.Error())
	}

	fileJsonBytes, ok := transMap["file"]
	if !ok {
		return shim.Error("file must be a key in the transient map")
	}

	if len(fileJsonBytes) == 0 {
		return shim.Error("file value in the transient map must be a non-empty JSON string")
	}

	var fileInput fileTransientInput
	err = json.Unmarshal(fileJsonBytes, &fileInput)
	if err != nil {
		return shim.Error("Failed to decode JSON of: " + string(fileJsonBytes))
	}

	if len(fileInput.Name) == 0 {
		return shim.Error("name field must be a non-empty string")
	}
	if len(fileInput.IPFShash) == 0 {
		return shim.Error("ipfshash field must be a non-empty string")
	}
	if fileInput.Timestamp <= 0 {
		return shim.Error("size field must be a positive integer")
	}
	if len(fileInput.Owner) == 0 {
		return shim.Error("owner field must be a non-empty string")
	}
	if fileInput.Folio <= 0 {
		return shim.Error("folio field must be a positive integer")
	}

	// ==== Check if file already exists ====
	fileAsBytes, err := stub.GetPrivateData("collectionFiles", fileInput.Name)
	if err != nil {
		return shim.Error("Failed to get file: " + err.Error())
	} else if fileAsBytes != nil {
		fmt.Println("This file already exists: " + fileInput.Name)
		return shim.Error("This file already exists: " + fileInput.Name)
	}

	// ==== Create file object and marshal to JSON ====
	file := &file{
		ObjectType: "file",
		Name:       fileInput.Name,
		IPFShash:      fileInput.IPFShash,
		Timestamp:       fileInput.Timestamp,
		Owner:      fileInput.Owner,
	}
	fileJSONasBytes, err := json.Marshal(file)
	if err != nil {
		return shim.Error(err.Error())
	}

	// === Save file to state ===
	err = stub.PutPrivateData("collectionFiles", fileInput.Name, fileJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	// ==== Create file private details object with folio, marshal to JSON, and save to state ====
	filePrivateDetails := &filePrivateDetails{
		ObjectType: "filePrivateDetails",
		Name:       fileInput.Name,
		Folio:      fileInput.Folio,
	}
	filePrivateDetailsBytes, err := json.Marshal(filePrivateDetails)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutPrivateData("collectionFilePrivateDetails", fileInput.Name, filePrivateDetailsBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	//  ==== Index the file to enable ipfshash-based range queries, e.g. return all blue files ====
	//  An 'index' is a normal key/value entry in state.
	//  The key is a composite key, with the elements that you want to range query on listed first.
	//  In our case, the composite key is based on indexName~ipfshash~name.
	//  This will enable very efficient state range queries based on composite keys matching indexName~ipfshash~*
	indexName := "ipfshash~name"
	ipfshashNameIndexKey, err := stub.CreateCompositeKey(indexName, []string{file.IPFShash, file.Name})
	if err != nil {
		return shim.Error(err.Error())
	}
	//  Save index entry to state. Only the key name is needed, no need to store a duplicate copy of the file.
	//  Note - passing a 'nil' value will effectively delete the key from state, therefore we pass null character as value
	value := []byte{0x00}
	stub.PutPrivateData("collectionFiles", ipfshashNameIndexKey, value)

	// ==== File saved and indexed. Return success ====
	fmt.Println("- end init file")
	return shim.Success(nil)
}

// ===============================================
// readFile - read a file from chaincode state
// ===============================================
func (t *FilesPrivateChaincode) readFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the file to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetPrivateData("collectionFiles", name) //get the file from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + name + ": " + err.Error() + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"File does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

// ===============================================
// readFilereadFilePrivateDetails - read a file private details from chaincode state
// ===============================================
func (t *FilesPrivateChaincode) readFilePrivateDetails(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the file to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetPrivateData("collectionFilePrivateDetails", name) //get the file private details from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get private details for " + name + ": " + err.Error() + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"File private details does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

// ===============================================
// getFileHash - get file private data hash for collectionFiles from chaincode state
// ===============================================
func (t *FilesPrivateChaincode) getFileHash(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the file to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetPrivateDataHash("collectionFiles", name)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get file private data hash for " + name + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"File private file data hash does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

// ===============================================
// getFilePrivateDetailsHash - get file private data hash for collectionFilePrivateDetails from chaincode state
// ===============================================
func (t *FilesPrivateChaincode) getFilePrivateDetailsHash(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the file to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetPrivateDataHash("collectionFilePrivateDetails", name)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get file private details hash for " + name + ": " + err.Error() + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"File private details hash does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

// ==================================================
// delete - remove a file key/value pair from state
// ==================================================
func (t *FilesPrivateChaincode) delete(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	fmt.Println("- start delete file")

	type fileDeleteTransientInput struct {
		Name string `json:"name"`
	}

	if len(args) != 0 {
		return shim.Error("Incorrect number of arguments. Private file name must be passed in transient map.")
	}

	transMap, err := stub.GetTransient()
	if err != nil {
		return shim.Error("Error getting transient: " + err.Error())
	}

	fileDeleteJsonBytes, ok := transMap["file_delete"]
	if !ok {
		return shim.Error("file_delete must be a key in the transient map")
	}

	if len(fileDeleteJsonBytes) == 0 {
		return shim.Error("file_delete value in the transient map must be a non-empty JSON string")
	}

	var fileDeleteInput fileDeleteTransientInput
	err = json.Unmarshal(fileDeleteJsonBytes, &fileDeleteInput)
	if err != nil {
		return shim.Error("Failed to decode JSON of: " + string(fileDeleteJsonBytes))
	}

	if len(fileDeleteInput.Name) == 0 {
		return shim.Error("name field must be a non-empty string")
	}

	// to maintain the ipfshash~name index, we need to read the file first and get its ipfshash
	valAsbytes, err := stub.GetPrivateData("collectionFiles", fileDeleteInput.Name) //get the file from chaincode state
	if err != nil {
		return shim.Error("Failed to get state for " + fileDeleteInput.Name)
	} else if valAsbytes == nil {
		return shim.Error("File does not exist: " + fileDeleteInput.Name)
	}

	var fileToDelete file
	err = json.Unmarshal([]byte(valAsbytes), &fileToDelete)
	if err != nil {
		return shim.Error("Failed to decode JSON of: " + string(valAsbytes))
	}

	// delete the file from state
	err = stub.DelPrivateData("collectionFiles", fileDeleteInput.Name)
	if err != nil {
		return shim.Error("Failed to delete state:" + err.Error())
	}

	// Also delete the file from the ipfshash~name index
	indexName := "ipfshash~name"
	ipfshashNameIndexKey, err := stub.CreateCompositeKey(indexName, []string{fileToDelete.IPFShash, fileToDelete.Name})
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.DelPrivateData("collectionFiles", ipfshashNameIndexKey)
	if err != nil {
		return shim.Error("Failed to delete state:" + err.Error())
	}

	// Finally, delete private details of file
	err = stub.DelPrivateData("collectionFilePrivateDetails", fileDeleteInput.Name)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

// ===========================================================
// transfer a file by setting a new owner name on the file
// ===========================================================
func (t *FilesPrivateChaincode) transferFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	fmt.Println("- start transfer file")

	type fileTransferTransientInput struct {
		Name  string `json:"name"`
		Owner string `json:"owner"`
	}

	if len(args) != 0 {
		return shim.Error("Incorrect number of arguments. Private file data must be passed in transient map.")
	}

	transMap, err := stub.GetTransient()
	if err != nil {
		return shim.Error("Error getting transient: " + err.Error())
	}

	fileOwnerJsonBytes, ok := transMap["file_owner"]
	if !ok {
		return shim.Error("file_owner must be a key in the transient map")
	}

	if len(fileOwnerJsonBytes) == 0 {
		return shim.Error("file_owner value in the transient map must be a non-empty JSON string")
	}

	var fileTransferInput fileTransferTransientInput
	err = json.Unmarshal(fileOwnerJsonBytes, &fileTransferInput)
	if err != nil {
		return shim.Error("Failed to decode JSON of: " + string(fileOwnerJsonBytes))
	}

	if len(fileTransferInput.Name) == 0 {
		return shim.Error("name field must be a non-empty string")
	}
	if len(fileTransferInput.Owner) == 0 {
		return shim.Error("owner field must be a non-empty string")
	}

	fileAsBytes, err := stub.GetPrivateData("collectionFiles", fileTransferInput.Name)
	if err != nil {
		return shim.Error("Failed to get file:" + err.Error())
	} else if fileAsBytes == nil {
		return shim.Error("File does not exist: " + fileTransferInput.Name)
	}

	fileToTransfer := file{}
	err = json.Unmarshal(fileAsBytes, &fileToTransfer) //unmarshal it aka JSON.parse()
	if err != nil {
		return shim.Error(err.Error())
	}
	fileToTransfer.Owner = fileTransferInput.Owner //change the owner

	fileJSONasBytes, _ := json.Marshal(fileToTransfer)
	err = stub.PutPrivateData("collectionFiles", fileToTransfer.Name, fileJSONasBytes) //rewrite the file
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("- end transferFile (success)")
	return shim.Success(nil)
}

// ===========================================================================================
// getFilesByRange performs a range query based on the start and end keys provided.

// Read-only function results are not typically submitted to ordering. If the read-only
// results are submitted to ordering, or if the query is used in an update transaction
// and submitted to ordering, then the committing peers will re-execute to guarantee that
// result sets are stable between endorsement time and commit time. The transaction is
// invalidated by the committing peers if the result set has changed between endorsement
// time and commit time.
// Therefore, range queries are a safe option for performing update transactions based on query results.
// ===========================================================================================
func (t *FilesPrivateChaincode) getFilesByRange(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) < 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}

	startKey := args[0]
	endKey := args[1]

	resultsIterator, err := stub.GetPrivateDataByRange("collectionFiles", startKey, endKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten {
			buffer.WriteString(",")
		}

		buffer.WriteString(
			fmt.Sprintf(
				`{"Key":"%s", "Record":%s}`,
				queryResponse.Key, queryResponse.Value,
			),
		)
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getFilesByRange queryResult:\n%s\n", buffer.String())

	return shim.Success(buffer.Bytes())
}

func main() {
	err := shim.Start(&FilesPrivateChaincode{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exiting Simple chaincode: %s", err)
		os.Exit(2)
	}
}
