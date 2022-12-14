package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	"github.com/TrueCloudLab/frostfs-http-gw/response"
	"github.com/TrueCloudLab/frostfs-http-gw/tokens"
	"github.com/TrueCloudLab/frostfs-http-gw/utils"
	"github.com/TrueCloudLab/frostfs-sdk-go/bearer"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
	"github.com/valyala/fasthttp"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

const (
	jsonHeader   = "application/json; charset=UTF-8"
	drainBufSize = 4096
)

// Uploader is an upload request handler.
type Uploader struct {
	appCtx            context.Context
	log               *zap.Logger
	pool              *pool.Pool
	ownerID           *user.ID
	settings          *Settings
	containerResolver *resolver.ContainerResolver
}

type epochDurations struct {
	currentEpoch  uint64
	msPerBlock    int64
	blockPerEpoch uint64
}

// Settings stores reloading parameters, so it has to provide atomic getters and setters.
type Settings struct {
	defaultTimestamp atomic.Bool
}

func (s *Settings) DefaultTimestamp() bool {
	return s.defaultTimestamp.Load()
}

func (s *Settings) SetDefaultTimestamp(val bool) {
	s.defaultTimestamp.Store(val)
}

// New creates a new Uploader using specified logger, connection pool and
// other options.
func New(ctx context.Context, params *utils.AppParams, settings *Settings) *Uploader {
	return &Uploader{
		appCtx:            ctx,
		log:               params.Logger,
		pool:              params.Pool,
		ownerID:           params.Owner,
		settings:          settings,
		containerResolver: params.Resolver,
	}
}

// Upload handles multipart upload request.
func (u *Uploader) Upload(c *fasthttp.RequestCtx) {
	var (
		file       MultipartFile
		idObj      oid.ID
		addr       oid.Address
		scid, _    = c.UserValue("cid").(string)
		log        = u.log.With(zap.String("cid", scid))
		bodyStream = c.RequestBodyStream()
		drainBuf   = make([]byte, drainBufSize)
	)

	if err := tokens.StoreBearerToken(c); err != nil {
		log.Error("could not fetch bearer token", zap.Error(err))
		response.Error(c, "could not fetch bearer token", fasthttp.StatusBadRequest)
		return
	}

	idCnr, err := utils.GetContainerID(u.appCtx, scid, u.containerResolver)
	if err != nil {
		log.Error("wrong container id", zap.Error(err))
		response.Error(c, "wrong container id", fasthttp.StatusBadRequest)
		return
	}

	defer func() {
		// If the temporary reader can be closed - let's close it.
		if file == nil {
			return
		}
		err := file.Close()
		log.Debug(
			"close temporary multipart/form file",
			zap.Stringer("address", addr),
			zap.String("filename", file.FileName()),
			zap.Error(err),
		)
	}()
	boundary := string(c.Request.Header.MultipartFormBoundary())
	if file, err = fetchMultipartFile(u.log, bodyStream, boundary); err != nil {
		log.Error("could not receive multipart/form", zap.Error(err))
		response.Error(c, "could not receive multipart/form: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}
	filtered, err := filterHeaders(u.log, &c.Request.Header)
	if err != nil {
		log.Error("could not process headers", zap.Error(err))
		response.Error(c, err.Error(), fasthttp.StatusBadRequest)
		return
	}
	if needParseExpiration(filtered) {
		epochDuration, err := getEpochDurations(c, u.pool)
		if err != nil {
			log.Error("could not get epoch durations from network info", zap.Error(err))
			response.Error(c, "could not get epoch durations from network info: "+err.Error(), fasthttp.StatusBadRequest)
			return
		}

		now := time.Now()
		if rawHeader := c.Request.Header.Peek(fasthttp.HeaderDate); rawHeader != nil {
			if parsed, err := time.Parse(http.TimeFormat, string(rawHeader)); err != nil {
				log.Warn("could not parse client time", zap.String("Date header", string(rawHeader)), zap.Error(err))
			} else {
				now = parsed
			}
		}

		if err = prepareExpirationHeader(filtered, epochDuration, now); err != nil {
			log.Error("could not parse expiration header", zap.Error(err))
			response.Error(c, "could not parse expiration header: "+err.Error(), fasthttp.StatusBadRequest)
			return
		}
	}

	attributes := make([]object.Attribute, 0, len(filtered))
	// prepares attributes from filtered headers
	for key, val := range filtered {
		attribute := object.NewAttribute()
		attribute.SetKey(key)
		attribute.SetValue(val)
		attributes = append(attributes, *attribute)
	}
	// sets FileName attribute if it wasn't set from header
	if _, ok := filtered[object.AttributeFileName]; !ok {
		filename := object.NewAttribute()
		filename.SetKey(object.AttributeFileName)
		filename.SetValue(file.FileName())
		attributes = append(attributes, *filename)
	}
	// sets Timestamp attribute if it wasn't set from header and enabled by settings
	if _, ok := filtered[object.AttributeTimestamp]; !ok && u.settings.DefaultTimestamp() {
		timestamp := object.NewAttribute()
		timestamp.SetKey(object.AttributeTimestamp)
		timestamp.SetValue(strconv.FormatInt(time.Now().Unix(), 10))
		attributes = append(attributes, *timestamp)
	}
	id, bt := u.fetchOwnerAndBearerToken(c)

	obj := object.New()
	obj.SetContainerID(*idCnr)
	obj.SetOwnerID(id)
	obj.SetAttributes(attributes...)

	var prm pool.PrmObjectPut
	prm.SetHeader(*obj)
	prm.SetPayload(file)

	if bt != nil {
		prm.UseBearer(*bt)
	}

	if idObj, err = u.pool.PutObject(u.appCtx, prm); err != nil {
		log.Error("could not store file in neofs", zap.Error(err))
		response.Error(c, "could not store file in neofs: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	addr.SetObject(idObj)
	addr.SetContainer(*idCnr)

	// Try to return the response, otherwise, if something went wrong, throw an error.
	if err = newPutResponse(addr).encode(c); err != nil {
		log.Error("could not encode response", zap.Error(err))
		response.Error(c, "could not encode response", fasthttp.StatusBadRequest)

		return
	}
	// Multipart is multipart and thus can contain more than one part which
	// we ignore at the moment. Also, when dealing with chunked encoding
	// the last zero-length chunk might be left unread (because multipart
	// reader only cares about its boundary and doesn't look further) and
	// it will be (erroneously) interpreted as the start of the next
	// pipelined header. Thus we need to drain the body buffer.
	for {
		_, err = bodyStream.Read(drainBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}
	// Report status code and content type.
	c.Response.SetStatusCode(fasthttp.StatusOK)
	c.Response.Header.SetContentType(jsonHeader)
}

func (u *Uploader) fetchOwnerAndBearerToken(ctx context.Context) (*user.ID, *bearer.Token) {
	if tkn, err := tokens.LoadBearerToken(ctx); err == nil && tkn != nil {
		issuer := bearer.ResolveIssuer(*tkn)
		return &issuer, tkn
	}
	return u.ownerID, nil
}

type putResponse struct {
	ObjectID    string `json:"object_id"`
	ContainerID string `json:"container_id"`
}

func newPutResponse(addr oid.Address) *putResponse {
	return &putResponse{
		ObjectID:    addr.Object().EncodeToString(),
		ContainerID: addr.Container().EncodeToString(),
	}
}

func (pr *putResponse) encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return enc.Encode(pr)
}

func getEpochDurations(ctx context.Context, p *pool.Pool) (*epochDurations, error) {
	networkInfo, err := p.NetworkInfo(ctx)
	if err != nil {
		return nil, err
	}

	res := &epochDurations{
		currentEpoch:  networkInfo.CurrentEpoch(),
		msPerBlock:    networkInfo.MsPerBlock(),
		blockPerEpoch: networkInfo.EpochDuration(),
	}

	if res.blockPerEpoch == 0 {
		return nil, fmt.Errorf("EpochDuration is empty")
	}
	return res, nil
}

func needParseExpiration(headers map[string]string) bool {
	_, ok1 := headers[utils.ExpirationDurationAttr]
	_, ok2 := headers[utils.ExpirationRFC3339Attr]
	_, ok3 := headers[utils.ExpirationTimestampAttr]
	return ok1 || ok2 || ok3
}
