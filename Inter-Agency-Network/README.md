#Smile-prerequisites
#Hyperledger Labs & Tong Li, IBM Open Technology

curl -o minifab -sL https://tinyurl.com/yxa2q6yr && chmod +x minifab

#Run Fabric

minifab up

#Fabric Shutdown
minifab down

#Get rid of Everything
minifab cleanup

#Run Explorer

minifab explorerup

#Explorer Shutdown
minifab explorerdown


#Quick Notes for ENV
# vars/envsettings

#!/bin/bash
declare XX_CHANNEL_NAME='inter-agency'
declare XX_CC_LANGUAGE='go'
declare XX_IMAGETAG='2.3.0'
declare XX_BLOCK_NUMBER='newest'
declare XX_CC_VERSION='1.0'
declare XX_CC_NAME='simple'
declare XX_DB_TYPE='golevel'
declare XX_CC_PARAMETERS='InF1ZXJ5IiwgImEiCg=='
declare XX_EXPOSE_ENDPOINTS='false'
declare XX_CURRENT_ORG='org0.unicef.com'
declare XX_TRANSIENT_DATA='Cg=='
declare XX_CC_PRIVATE='true'
declare XX_CC_POLICY='Cg=='
declare XX_CC_INIT_REQUIRED='true'
declare XX_RUN_OUTPUT=''

#privatefiles chaincode concept "forked from IBM"
# under main.go
