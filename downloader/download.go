package downloader

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	"github.com/TrueCloudLab/frostfs-http-gw/response"
	"github.com/TrueCloudLab/frostfs-http-gw/tokens"
	"github.com/TrueCloudLab/frostfs-http-gw/utils"
	"github.com/TrueCloudLab/frostfs-sdk-go/bearer"
	"github.com/TrueCloudLab/frostfs-sdk-go/client"
	"github.com/TrueCloudLab/frostfs-sdk-go/container"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
	"github.com/valyala/fasthttp"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type request struct {
	*fasthttp.RequestCtx
	appCtx context.Context
	log    *zap.Logger
}

func isValidToken(s string) bool {
	for _, c := range s {
		if c <= ' ' || c > 127 {
			return false
		}
		if strings.ContainsRune("()<>@,;:\\\"/[]?={}", c) {
			return false
		}
	}
	return true
}

func isValidValue(s string) bool {
	for _, c := range s {
		// HTTP specification allows for more technically, but we don't want to escape things.
		if c < ' ' || c > 127 || c == '"' {
			return false
		}
	}
	return true
}

type readCloser struct {
	io.Reader
	io.Closer
}

// initializes io.Reader with the limited size and detects Content-Type from it.
// Returns r's error directly. Also returns the processed data.
func readContentType(maxSize uint64, rInit func(uint64) (io.Reader, error)) (string, []byte, error) {
	if maxSize > sizeToDetectType {
		maxSize = sizeToDetectType
	}

	buf := make([]byte, maxSize) // maybe sync-pool the slice?

	r, err := rInit(maxSize)
	if err != nil {
		return "", nil, err
	}

	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return "", nil, err
	}

	buf = buf[:n]

	return http.DetectContentType(buf), buf, err // to not lose io.EOF
}

func (r request) receiveFile(clnt *pool.Pool, objectAddress oid.Address) {
	var (
		err      error
		dis      = "inline"
		start    = time.Now()
		filename string
	)
	if err = tokens.StoreBearerToken(r.RequestCtx); err != nil {
		r.log.Error("could not fetch and store bearer token", zap.Error(err))
		response.Error(r.RequestCtx, "could not fetch and store bearer token: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	var prm pool.PrmObjectGet
	prm.SetAddress(objectAddress)
	if btoken := bearerToken(r.RequestCtx); btoken != nil {
		prm.UseBearer(*btoken)
	}

	rObj, err := clnt.GetObject(r.appCtx, prm)
	if err != nil {
		r.handleNeoFSErr(err, start)
		return
	}

	// we can't close reader in this function, so how to do it?

	if r.Request.URI().QueryArgs().GetBool("download") {
		dis = "attachment"
	}

	payloadSize := rObj.Header.PayloadSize()

	r.Response.Header.Set(fasthttp.HeaderContentLength, strconv.FormatUint(payloadSize, 10))
	var contentType string
	for _, attr := range rObj.Header.Attributes() {
		key := attr.Key()
		val := attr.Value()
		if !isValidToken(key) || !isValidValue(val) {
			continue
		}
		if strings.HasPrefix(key, utils.SystemAttributePrefix) {
			key = systemBackwardTranslator(key)
		}
		r.Response.Header.Set(utils.UserAttributeHeaderPrefix+key, val)
		switch key {
		case object.AttributeFileName:
			filename = val
		case object.AttributeTimestamp:
			value, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				r.log.Info("couldn't parse creation date",
					zap.String("key", key),
					zap.String("val", val),
					zap.Error(err))
				continue
			}
			r.Response.Header.Set(fasthttp.HeaderLastModified,
				time.Unix(value, 0).UTC().Format(http.TimeFormat))
		case object.AttributeContentType:
			contentType = val
		}
	}

	idsToResponse(&r.Response, &rObj.Header)

	if len(contentType) == 0 {
		// determine the Content-Type from the payload head
		var payloadHead []byte

		contentType, payloadHead, err = readContentType(payloadSize, func(uint64) (io.Reader, error) {
			return rObj.Payload, nil
		})
		if err != nil && err != io.EOF {
			r.log.Error("could not detect Content-Type from payload", zap.Error(err))
			response.Error(r.RequestCtx, "could not detect Content-Type from payload: "+err.Error(), fasthttp.StatusBadRequest)
			return
		}

		// reset payload reader since a part of the data has been read
		var headReader io.Reader = bytes.NewReader(payloadHead)

		if err != io.EOF { // otherwise, we've already read full payload
			headReader = io.MultiReader(headReader, rObj.Payload)
		}

		// note: we could do with io.Reader, but SetBodyStream below closes body stream
		// if it implements io.Closer and that's useful for us.
		rObj.Payload = readCloser{headReader, rObj.Payload}
	}
	r.SetContentType(contentType)

	r.Response.Header.Set(fasthttp.HeaderContentDisposition, dis+"; filename="+path.Base(filename))

	r.Response.SetBodyStream(rObj.Payload, int(payloadSize))
}

// systemBackwardTranslator is used to convert headers looking like '__NEOFS__ATTR_NAME' to 'Neofs-Attr-Name'.
func systemBackwardTranslator(key string) string {
	// trim specified prefix '__NEOFS__'
	key = strings.TrimPrefix(key, utils.SystemAttributePrefix)

	var res strings.Builder
	res.WriteString("Neofs-")

	strs := strings.Split(key, "_")
	for i, s := range strs {
		s = title(strings.ToLower(s))
		res.WriteString(s)
		if i != len(strs)-1 {
			res.WriteString("-")
		}
	}

	return res.String()
}

func title(str string) string {
	if str == "" {
		return ""
	}

	r, size := utf8.DecodeRuneInString(str)
	r0 := unicode.ToTitle(r)
	return string(r0) + str[size:]
}

func bearerToken(ctx context.Context) *bearer.Token {
	if tkn, err := tokens.LoadBearerToken(ctx); err == nil {
		return tkn
	}
	return nil
}

func (r *request) handleNeoFSErr(err error, start time.Time) {
	r.log.Error(
		"could not receive object",
		zap.Stringer("elapsed", time.Since(start)),
		zap.Error(err),
	)

	if client.IsErrObjectNotFound(err) || client.IsErrContainerNotFound(err) {
		response.Error(r.RequestCtx, "Not Found", fasthttp.StatusNotFound)
		return
	}

	msg := fmt.Sprintf("could not receive object: %v", err)
	response.Error(r.RequestCtx, msg, fasthttp.StatusBadRequest)
}

// Downloader is a download request handler.
type Downloader struct {
	appCtx            context.Context
	log               *zap.Logger
	pool              *pool.Pool
	containerResolver *resolver.ContainerResolver
	settings          *Settings
}

// Settings stores reloading parameters, so it has to provide atomic getters and setters.
type Settings struct {
	zipCompression atomic.Bool
}

func (s *Settings) ZipCompression() bool {
	return s.zipCompression.Load()
}

func (s *Settings) SetZipCompression(val bool) {
	s.zipCompression.Store(val)
}

// New creates an instance of Downloader using specified options.
func New(ctx context.Context, params *utils.AppParams, settings *Settings) *Downloader {
	return &Downloader{
		appCtx:            ctx,
		log:               params.Logger,
		pool:              params.Pool,
		settings:          settings,
		containerResolver: params.Resolver,
	}
}

func (d *Downloader) newRequest(ctx *fasthttp.RequestCtx, log *zap.Logger) *request {
	return &request{
		RequestCtx: ctx,
		appCtx:     d.appCtx,
		log:        log,
	}
}

// DownloadByAddress handles download requests using simple cid/oid format.
func (d *Downloader) DownloadByAddress(c *fasthttp.RequestCtx) {
	d.byAddress(c, request.receiveFile)
}

// byAddress is a wrapper for function (e.g. request.headObject, request.receiveFile) that
// prepares request and object address to it.
func (d *Downloader) byAddress(c *fasthttp.RequestCtx, f func(request, *pool.Pool, oid.Address)) {
	var (
		idCnr, _ = c.UserValue("cid").(string)
		idObj, _ = c.UserValue("oid").(string)
		log      = d.log.With(zap.String("cid", idCnr), zap.String("oid", idObj))
	)

	cnrID, err := utils.GetContainerID(d.appCtx, idCnr, d.containerResolver)
	if err != nil {
		log.Error("wrong container id", zap.Error(err))
		response.Error(c, "wrong container id", fasthttp.StatusBadRequest)
		return
	}

	objID := new(oid.ID)
	if err = objID.DecodeString(idObj); err != nil {
		log.Error("wrong object id", zap.Error(err))
		response.Error(c, "wrong object id", fasthttp.StatusBadRequest)
		return
	}

	var addr oid.Address
	addr.SetContainer(*cnrID)
	addr.SetObject(*objID)

	f(*d.newRequest(c, log), d.pool, addr)
}

// DownloadByAttribute handles attribute-based download requests.
func (d *Downloader) DownloadByAttribute(c *fasthttp.RequestCtx) {
	d.byAttribute(c, request.receiveFile)
}

// byAttribute is a wrapper similar to byAddress.
func (d *Downloader) byAttribute(c *fasthttp.RequestCtx, f func(request, *pool.Pool, oid.Address)) {
	var (
		scid, _ = c.UserValue("cid").(string)
		key, _  = url.QueryUnescape(c.UserValue("attr_key").(string))
		val, _  = url.QueryUnescape(c.UserValue("attr_val").(string))
		log     = d.log.With(zap.String("cid", scid), zap.String("attr_key", key), zap.String("attr_val", val))
	)

	containerID, err := utils.GetContainerID(d.appCtx, scid, d.containerResolver)
	if err != nil {
		log.Error("wrong container id", zap.Error(err))
		response.Error(c, "wrong container id", fasthttp.StatusBadRequest)
		return
	}

	res, err := d.search(c, containerID, key, val, object.MatchStringEqual)
	if err != nil {
		log.Error("could not search for objects", zap.Error(err))
		response.Error(c, "could not search for objects: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	defer res.Close()

	buf := make([]oid.ID, 1)

	n, err := res.Read(buf)
	if n == 0 {
		if errors.Is(err, io.EOF) {
			log.Error("object not found", zap.Error(err))
			response.Error(c, "object not found", fasthttp.StatusNotFound)
			return
		}

		log.Error("read object list failed", zap.Error(err))
		response.Error(c, "read object list failed: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	var addrObj oid.Address
	addrObj.SetContainer(*containerID)
	addrObj.SetObject(buf[0])

	f(*d.newRequest(c, log), d.pool, addrObj)
}

func (d *Downloader) search(c *fasthttp.RequestCtx, cid *cid.ID, key, val string, op object.SearchMatchType) (pool.ResObjectSearch, error) {
	filters := object.NewSearchFilters()
	filters.AddRootFilter()
	filters.AddFilter(key, val, op)

	var prm pool.PrmObjectSearch
	prm.SetContainerID(*cid)
	prm.SetFilters(filters)
	if btoken := bearerToken(c); btoken != nil {
		prm.UseBearer(*btoken)
	}

	return d.pool.SearchObjects(d.appCtx, prm)
}

func (d *Downloader) getContainer(cnrID cid.ID) (container.Container, error) {
	var prm pool.PrmContainerGet
	prm.SetContainerID(cnrID)

	return d.pool.GetContainer(d.appCtx, prm)
}

func (d *Downloader) addObjectToZip(zw *zip.Writer, obj *object.Object) (io.Writer, error) {
	method := zip.Store
	if d.settings.ZipCompression() {
		method = zip.Deflate
	}

	filePath := getZipFilePath(obj)
	if len(filePath) == 0 || filePath[len(filePath)-1] == '/' {
		return nil, fmt.Errorf("invalid filepath '%s'", filePath)
	}

	return zw.CreateHeader(&zip.FileHeader{
		Name:     filePath,
		Method:   method,
		Modified: time.Now(),
	})
}

// DownloadZipped handles zip by prefix requests.
func (d *Downloader) DownloadZipped(c *fasthttp.RequestCtx) {
	scid, _ := c.UserValue("cid").(string)
	prefix, _ := url.QueryUnescape(c.UserValue("prefix").(string))
	log := d.log.With(zap.String("cid", scid), zap.String("prefix", prefix))

	containerID, err := utils.GetContainerID(d.appCtx, scid, d.containerResolver)
	if err != nil {
		log.Error("wrong container id", zap.Error(err))
		response.Error(c, "wrong container id", fasthttp.StatusBadRequest)
		return
	}

	if err = tokens.StoreBearerToken(c); err != nil {
		log.Error("could not fetch and store bearer token", zap.Error(err))
		response.Error(c, "could not fetch and store bearer token: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// check if container exists here to be able to return 404 error,
	// otherwise we get this error only in object iteration step
	// and client get 200 OK.
	if _, err = d.getContainer(*containerID); err != nil {
		log.Error("could not check container existence", zap.Error(err))
		if client.IsErrContainerNotFound(err) {
			response.Error(c, "Not Found", fasthttp.StatusNotFound)
			return
		}
		response.Error(c, "could not check container existence: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	resSearch, err := d.search(c, containerID, object.AttributeFilePath, prefix, object.MatchCommonPrefix)
	if err != nil {
		log.Error("could not search for objects", zap.Error(err))
		response.Error(c, "could not search for objects: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	c.Response.Header.Set(fasthttp.HeaderContentType, "application/zip")
	c.Response.Header.Set(fasthttp.HeaderContentDisposition, "attachment; filename=\"archive.zip\"")
	c.Response.SetStatusCode(http.StatusOK)

	c.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer resSearch.Close()

		zipWriter := zip.NewWriter(w)

		var bufZip []byte
		var addr oid.Address

		empty := true
		called := false
		btoken := bearerToken(c)
		addr.SetContainer(*containerID)

		errIter := resSearch.Iterate(func(id oid.ID) bool {
			called = true

			if empty {
				bufZip = make([]byte, 3<<20) // the same as for upload
			}
			empty = false

			addr.SetObject(id)
			if err = d.zipObject(zipWriter, addr, btoken, bufZip); err != nil {
				log.Error("failed to add object to archive", zap.String("oid", id.EncodeToString()), zap.Error(err))
			}

			return false
		})
		if errIter != nil {
			log.Error("iterating over selected objects failed", zap.Error(errIter))
		} else if !called {
			log.Error("objects not found")
		}

		if err = zipWriter.Close(); err != nil {
			log.Error("close zip writer", zap.Error(err))
		}
	})
}

func (d *Downloader) zipObject(zipWriter *zip.Writer, addr oid.Address, btoken *bearer.Token, bufZip []byte) error {
	var prm pool.PrmObjectGet
	prm.SetAddress(addr)
	if btoken != nil {
		prm.UseBearer(*btoken)
	}

	resGet, err := d.pool.GetObject(d.appCtx, prm)
	if err != nil {
		return fmt.Errorf("get NeoFS object: %v", err)
	}

	objWriter, err := d.addObjectToZip(zipWriter, &resGet.Header)
	if err != nil {
		return fmt.Errorf("zip create header: %v", err)
	}

	if _, err = io.CopyBuffer(objWriter, resGet.Payload, bufZip); err != nil {
		return fmt.Errorf("copy object payload to zip file: %v", err)
	}

	if err = resGet.Payload.Close(); err != nil {
		return fmt.Errorf("object body close error: %w", err)
	}

	if err = zipWriter.Flush(); err != nil {
		return fmt.Errorf("flush zip writer: %v", err)
	}

	return nil
}

func getZipFilePath(obj *object.Object) string {
	for _, attr := range obj.Attributes() {
		if attr.Key() == object.AttributeFilePath {
			return attr.Value()
		}
	}

	return ""
}
