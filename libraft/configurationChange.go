package libraft

type AddRemoveServerError string

const AR_NOT_LEADER = AddRemoveServerError("NOT_LEADER")
const AR_NOT_PROVISIONAL = AddRemoveServerError("NOT_PROVISIONAL")
const AR_NOT_CAUGHT_UP = AddRemoveServerError("NOT_CAUGHT_UP")
const AR_CONCURRENT_CHANGE = AddRemoveServerError("CONCURRENT_CHANGE")
const AR_ALREADY_COMPLETE = AddRemoveServerError("ALREADY_IN_CONFIGURATION")
const AR_NEW_LEADER = AddRemoveServerError("NEW_LEADER")
const AR_OK = AddRemoveServerError("OK")

type AddProvisionalError string

const AP_NOT_LEADER = AddProvisionalError("NOT_LEADER")
const AP_ALREADY_PROVISIONAL = AddProvisionalError("ALREADY_PROVISIONAL")
const AP_ALREADY_IN_CONFIGURATION = AddProvisionalError("ALREADY_IN_CONFIGURATION")
const AP_SUCCESS = AddProvisionalError("SUCCESS")
