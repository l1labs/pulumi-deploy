package aws

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/acm"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// HTTPS is the struct for creating HTTPS certs and associated DNS records
type HTTPS struct {
	Name                    string
	Zone                    string
	PrivateZone             bool
	DomainName              string
	SubjectAlternativeNames []string

	Out struct {
		Cert   *acm.Certificate
		Record *route53.Record
		Zone   *route53.LookupZoneResult
	}
}

func (s *HTTPS) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing HTTPS.Name")
	}

	if s.Zone == "" {
		return fmt.Errorf("missing HTTPS.Zone")
	}

	if !strings.HasSuffix(s.Zone, ".") {
		return fmt.Errorf("HTTPS.Zone <%v> is invalid - must end with trailing period, i.e. <domain.com.>", s.Zone)
	}

	if s.DomainName == "" {
		return fmt.Errorf("missing HTTPS.DomainName")
	}

	return nil
}

func (s *HTTPS) Run(ctx *pulumi.Context) error {
	if err := s.Validate(); err != nil {
		return err
	}

	certName := fmt.Sprintf("%v-cert", s.Name)
	args := &acm.CertificateArgs{
		DomainName:       pulumi.String(s.DomainName),
		Tags:             pulumi.StringMap{},
		ValidationMethod: pulumi.String("DNS"),
	}

	if len(s.SubjectAlternativeNames) > 0 {
		values := make(pulumi.StringArray, len(s.SubjectAlternativeNames))
		for idx, v := range s.SubjectAlternativeNames {
			values[idx] = pulumi.String(v)
		}
		args.SubjectAlternativeNames = values
	}

	cert, err := acm.NewCertificate(ctx, certName, args)
	if err != nil {
		return err
	}

	validation := cert.DomainValidationOptions.Index(pulumi.Int(0))
	s.Out.Cert = cert

	zone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
		Name:        &s.Zone,
		PrivateZone: &s.PrivateZone,
	})
	if err != nil {
		return err
	}
	s.Out.Zone = zone

	recordName := validation.ResourceRecordName().ApplyT(
		func(value interface{}) (string, error) {
			extracted, ok := value.(*string)
			if !ok {
				return "", fmt.Errorf("unable to coerce %v", value)
			}

			return *extracted, nil
		},
	).(pulumi.StringOutput)

	recordValue := validation.ResourceRecordValue().ApplyT(
		func(value interface{}) (string, error) {

			extracted, ok := value.(*string)
			if !ok {
				return "", fmt.Errorf("unable to coerce %v", value)
			}

			return *extracted, nil
		},
	).(pulumi.StringOutput)

	urlName := fmt.Sprintf("%v-url", s.Name)
	record, err := route53.NewRecord(ctx, urlName, &route53.RecordArgs{
		ZoneId: pulumi.String(zone.ZoneId),
		Name:   recordName,
		Type:   validation.ResourceRecordType().Elem().ToStringOutput(),
		Ttl:    pulumi.Int(300),
		Records: pulumi.StringArray{
			recordValue,
		},
	})
	if err != nil {
		return err
	}
	s.Out.Record = record

	if len(s.SubjectAlternativeNames) > 0 {
		for i := 1; i < len(s.SubjectAlternativeNames)+1; i++ {
			validation := cert.DomainValidationOptions.Index(pulumi.Int(i))
			if err := s.validateSubjectAlternativeName(ctx, fmt.Sprintf("%v-%d", s.Name, i), zone, validation); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *HTTPS) validateSubjectAlternativeName(ctx *pulumi.Context, name string, zone *route53.LookupZoneResult, validation acm.CertificateDomainValidationOptionOutput) error {
	recordName := validation.ResourceRecordName().ApplyT(
		func(value interface{}) (string, error) {
			extracted, ok := value.(*string)
			if !ok {
				return "", fmt.Errorf("unable to coerce %v", value)
			}

			return *extracted, nil
		},
	).(pulumi.StringOutput)

	recordValue := validation.ResourceRecordValue().ApplyT(
		func(value interface{}) (string, error) {

			extracted, ok := value.(*string)
			if !ok {
				return "", fmt.Errorf("unable to coerce %v", value)
			}

			return *extracted, nil
		},
	).(pulumi.StringOutput)

	urlName := fmt.Sprintf("%v-subject-url", name)
	_, err := route53.NewRecord(ctx, urlName, &route53.RecordArgs{
		ZoneId: pulumi.String(zone.ZoneId),
		Name:   recordName,
		Type:   validation.ResourceRecordType().Elem().ToStringOutput(),
		Ttl:    pulumi.Int(300),
		Records: pulumi.StringArray{
			recordValue,
		},
	})

	if err != nil {
		return err
	}

	return nil
}
