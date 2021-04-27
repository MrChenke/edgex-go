package http

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	dataContainer "github.com/edgexfoundry/edgex-go/internal/core/data/container"
	"github.com/edgexfoundry/edgex-go/internal/core/data/v2/application"
	"github.com/edgexfoundry/edgex-go/internal/core/data/v2/io"
	"github.com/edgexfoundry/edgex-go/internal/pkg"
	"github.com/edgexfoundry/edgex-go/internal/pkg/v2/utils"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/v2"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/common"
	requestDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/requests"
	responseDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/responses"

	"github.com/gorilla/mux"
)

type EventController struct {
	readers map[string]io.EventReader
	dic     *di.Container
}

// NewEventController creates and initializes an EventController
func NewEventController(dic *di.Container) *EventController {
	return &EventController{
		readers: make(map[string]io.EventReader),
		dic:     dic,
	}
}

func (ec *EventController) getReader(r *http.Request) io.EventReader {
	contentType := strings.ToLower(r.Header.Get(clients.ContentType))
	if reader, ok := ec.readers[contentType]; ok {
		return reader
	}
	reader := io.NewEventRequestReader(contentType)
	ec.readers[contentType] = reader
	return reader
}

func (ec *EventController) AddEvent(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}

	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)

	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	profileName := vars[v2.ProfileName]
	deviceName := vars[v2.DeviceName]
	sourceName := vars[v2.SourceName]

	var addEventReqDTO requestDTO.AddEventRequest

	bytes, err := io.ReadDataInBytes(r.Body)
	if err == nil {
		// Per https://github.com/edgexfoundry/edgex-go/pull/3202#discussion_r587618347
		// V2 shall asynchronously publish initially encoded payload (not re-encoding) to message bus
		go application.PublishEvent(bytes, profileName, deviceName, sourceName, ctx, ec.dic)
		// unmarshal bytes to AddEventRequest
		reader := ec.getReader(r)
		addEventReqDTO, err = reader.ReadAddEventRequest(bytes)
	}
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	event := requestDTO.AddEventReqToEventModel(addEventReqDTO)
	err = application.ValidateEvent(event, profileName, deviceName, sourceName, ctx, ec.dic)
	if err == nil {
		err = application.AddEvent(event, ctx, ec.dic)
	}
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, addEventReqDTO.RequestId)
		return
	}

	response := commonDTO.NewBaseWithIdResponse(addEventReqDTO.RequestId, "", http.StatusCreated, event.Id)
	utils.WriteHttpHeader(w, ctx, http.StatusCreated)
	// encode and send out the response
	pkg.Encode(response, w, lc)
}

func (ec *EventController) EventById(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)

	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	id := vars[v2.Id]

	// Get the event
	e, err := application.EventById(id, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewEventResponse("", "", http.StatusOK, e)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	// encode and send out the response
	pkg.Encode(response, w, lc)
}

func (ec *EventController) DeleteEventById(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)

	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	id := vars[v2.Id]

	// Delete the event
	err := application.DeleteEventById(id, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewBaseResponse("", "", http.StatusOK)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	// encode and send out the response
	pkg.Encode(response, w, lc)
}

func (ec *EventController) EventTotalCount(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()

	// Count the event
	count, err := application.EventTotalCount(ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewCountResponse("", "", http.StatusOK, count)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc) // encode and send out the response
}

func (ec *EventController) EventCountByDeviceName(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	deviceName := vars[v2.Name]

	// Count the event by device
	count, err := application.EventCountByDeviceName(deviceName, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewCountResponse("", "", http.StatusOK, count)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc) // encode and send out the response
}

func (ec *EventController) AllEvents(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()
	config := dataContainer.ConfigurationFrom(ec.dic.Get)

	// parse URL query string for offset, limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	events, err := application.AllEvents(offset, limit, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiEventsResponse("", "", http.StatusOK, events)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (ec *EventController) EventsByDeviceName(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()
	config := dataContainer.ConfigurationFrom(ec.dic.Get)

	vars := mux.Vars(r)
	name := vars[v2.Name]

	// parse URL query string for offset, limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	events, err := application.EventsByDeviceName(offset, limit, name, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiEventsResponse("", "", http.StatusOK, events)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (ec *EventController) DeleteEventsByDeviceName(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)

	ctx := r.Context()
	vars := mux.Vars(r)
	deviceName := vars[v2.Name]

	// Delete events with associated Device deviceName
	err := application.DeleteEventsByDeviceName(deviceName, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewBaseResponse("", "", http.StatusAccepted)
	utils.WriteHttpHeader(w, ctx, http.StatusAccepted)
	// encode and send out the response
	pkg.Encode(response, w, lc)
}

func (ec *EventController) EventsByTimeRange(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()
	config := dataContainer.ConfigurationFrom(ec.dic.Get)

	// parse time range (start, end), offset, and limit from incoming request
	start, end, offset, limit, err := utils.ParseTimeRangeOffsetLimit(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	events, err := application.EventsByTimeRange(start, end, offset, limit, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiEventsResponse("", "", http.StatusOK, events)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (ec *EventController) DeleteEventsByAge(w http.ResponseWriter, r *http.Request) {
	// retrieve all the service injections from bootstrap
	lc := container.LoggingClientFrom(ec.dic.Get)
	ctx := r.Context()

	vars := mux.Vars(r)
	age, parsingErr := strconv.ParseInt(vars[v2.Age], 10, 64)

	if parsingErr != nil {
		err := errors.NewCommonEdgeX(errors.KindContractInvalid, "age format parsing failed", parsingErr)
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	err := application.DeleteEventsByAge(age, ec.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewBaseResponse("", "", http.StatusAccepted)
	utils.WriteHttpHeader(w, ctx, http.StatusAccepted)
	// encode and send out the response
	pkg.Encode(response, w, lc)
}
