package ddns

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"context"
	"fmt"
	"net/http"
	"strings"

	cfapi "github.com/cloudflare/cloudflare-go"
	"go.uber.org/zap"
)

type cloudflare struct {
	token string
	zones map[string]string
	ttl   int
}

type logger struct {
	ctx context.Context
}

type cloudflareHandle struct {
	ID     string
	ZoneID string
}

func (l *logger) Printf(format string, v ...interface{}) {
	log.S(l.ctx).Debugf(format, v)
}

func (d *cloudflare) getAPI(ctx context.Context) (*cfapi.API, error) {
	client := http.DefaultClient

	if ctxClient := ctx.Value(common.HttpClientKey); ctxClient != nil {
		client = ctxClient.(*http.Client)
	}

	api, err := cfapi.NewWithAPIToken(d.token, cfapi.HTTPClient(client), cfapi.UsingLogger(&logger{ctx: ctx}))
	if err != nil {
		log.S(ctx).Errorw("failed create cloudflare API", zap.Error(err))
		return nil, fmt.Errorf("failed create cloudflare API: %w", err)
	}

	return api, nil
}

func (d *cloudflare) getZoneResource(ctx context.Context, domain string) (*cfapi.ResourceContainer, error) {
	zoneID := ""
	for zone, id := range d.zones {
		if strings.HasSuffix(domain, zone) {
			zoneID = id
			break
		}
	}

	if zoneID == "" {
		log.S(ctx).Errorw("domain not belong to any zone", "domain", domain)
		return nil, fmt.Errorf("domain not belong to any zone")
	}

	return cfapi.ZoneIdentifier(zoneID), nil
}

func (d *cloudflare) FindRecord(ctx context.Context, r Record) (records []Record, err error) {
	ctx = log.SWith(ctx,
		"action", "find",
		"ns_type", r.Type,
		"domain", r.Domain,
		"mark", r.Mark)

	params := cfapi.ListDNSRecordsParams{
		Type:    r.Type,
		Name:    r.Domain,
		Comment: r.Mark,
	}

	api, err := d.getAPI(ctx)
	if err != nil {
		return nil, err
	}

	zoneRc, err := d.getZoneResource(ctx, r.Domain)
	if err != nil {
		return nil, err
	}

	cfRecords, info, err := api.ListDNSRecords(ctx, zoneRc, params)
	if err != nil {
		log.S(ctx).Errorw("failed list records", zap.Error(err))
		return nil, fmt.Errorf("failed list records: %w", err)
	}

	if info.HasMorePages() {
		log.S(ctx).Warnw("partial result, ignore remaining", "count", len(cfRecords), "total", info.Count, "pages", info.TotalPages)
	}

	for _, record := range cfRecords {
		records = append(records, Record{
			Handle:  cloudflareHandle{record.ID, zoneRc.Identifier},
			Domain:  record.Name,
			Type:    record.Type,
			Address: record.Content,
			Mark:    record.Comment,
		})
	}

	log.S(ctx).Debugw("find records", "records", records)

	return records, nil
}

func (d *cloudflare) WriteRecord(ctx context.Context, r Record) (Record, error) {
	pCtx := ctx
	ctx = log.SWith(ctx,
		"type", "cloudflare",
		"action", "write",
		"ns_type", r.Type,
		"domain", r.Domain,
		"address", r.Address,
		"handle", r.Handle,
		"mark", r.Mark)

	api, err := d.getAPI(ctx)
	if err != nil {
		return Record{}, err
	}

	var cfRecord cfapi.DNSRecord
	var zoneID string

	if r.Handle != nil {
		log.S(ctx).Debugw("updating record")
		handle := r.Handle.(cloudflareHandle)

		params := cfapi.UpdateDNSRecordParams{
			Type:    r.Type,
			Name:    r.Domain,
			Content: r.Address,
			ID:      handle.ID,
			Comment: &r.Mark,
		}

		zoneID = handle.ZoneID
		cfRecord, err = api.UpdateDNSRecord(ctx, cfapi.ZoneIdentifier(handle.ZoneID), params)
		if err != nil {
			log.S(ctx).Warnw("failed update record", zap.Error(err))
			return Record{}, fmt.Errorf("failed update record: %w", err)
		}
	} else {
		log.S(ctx).Debugw("creating record")

		zoneRc, err := d.getZoneResource(ctx, r.Domain)
		if err != nil {
			return Record{}, err
		}

		params := cfapi.CreateDNSRecordParams{
			Type:    r.Type,
			Name:    r.Domain,
			Content: r.Address,
			TTL:     d.ttl,
			Proxied: cfapi.BoolPtr(false),
			Comment: r.Mark,
		}

		cfRecord, err = api.CreateDNSRecord(ctx, zoneRc, params)
		zoneID = zoneRc.Identifier
		if err != nil {
			log.S(ctx).Warnw("failed create record", zap.Error(err))
			return Record{}, fmt.Errorf("failed create record: %w", err)
		}
	}

	record := Record{
		Handle: cloudflareHandle{
			ID:     cfRecord.ID,
			ZoneID: zoneID,
		},
		Domain:  cfRecord.Name,
		Type:    cfRecord.Type,
		Address: cfRecord.Content,
		Mark:    cfRecord.Comment,
	}

	log.S(pCtx).Debugw("record written", "record", record)

	return record, nil
}

func newCloudflare(ctx context.Context, provider config.CloudflareConfig) (_ Interface, err error) {
	ctx = log.SWith(ctx, "type", "cloudflare")

	c := provider
	d := &cloudflare{
		token: c.APIToken,
		zones: map[string]string{},
		ttl:   c.TTL,
	}

	api, err := d.getAPI(ctx)
	if err != nil {
		return nil, err
	}

	for _, name := range c.ZoneNames {
		id, err := api.ZoneIDByName(name)
		if err != nil {
			log.S(ctx).Errorw("failed get zone id", "zone", name, zap.Error(err))
			return nil, fmt.Errorf("failed get zone id: %w", err)
		}

		d.zones[name] = id
	}

	return d, nil
}
