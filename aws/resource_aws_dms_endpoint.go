package aws

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	dms "github.com/aws/aws-sdk-go/service/databasemigrationservice"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/keyvaluetags"
	tfdms "github.com/terraform-providers/terraform-provider-aws/aws/internal/service/dms"
)

func resourceAwsDmsEndpoint() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsDmsEndpointCreate,
		Read:   resourceAwsDmsEndpointRead,
		Update: resourceAwsDmsEndpointUpdate,
		Delete: resourceAwsDmsEndpointDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"certificate_arn": {
				Type:         schema.TypeString,
				Computed:     true,
				Optional:     true,
				ValidateFunc: validateArn,
			},
			"database_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"elasticsearch_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_uri": {
							Type:     schema.TypeString,
							Required: true,
							// API returns this error with ModifyEndpoint:
							// InvalidParameterCombinationException: Elasticsearch endpoint cant be modified.
							ForceNew: true,
						},
						"error_retry_duration": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      300,
							ValidateFunc: validation.IntAtLeast(0),
							// API returns this error with ModifyEndpoint:
							// InvalidParameterCombinationException: Elasticsearch endpoint cant be modified.
							ForceNew: true,
						},
						"full_load_error_percentage": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      10,
							ValidateFunc: validation.IntBetween(0, 100),
							// API returns this error with ModifyEndpoint:
							// InvalidParameterCombinationException: Elasticsearch endpoint cant be modified.
							ForceNew: true,
						},
						"service_access_role_arn": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validateArn,
							// API returns this error with ModifyEndpoint:
							// InvalidParameterCombinationException: Elasticsearch endpoint cant be modified.
							ForceNew: true,
						},
					},
				},
			},
			"endpoint_arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"endpoint_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateDmsEndpointId,
			},
			"endpoint_type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(dms.ReplicationEndpointTypeValue_Values(), false),
			},
			"engine_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(tfdms.EngineName_Values(), false),
			},
			"extra_connection_attributes": {
				Type:             schema.TypeString,
				Computed:         true,
				Optional:         true,
				DiffSuppressFunc: suppressExtraConnectionAttributesDiffs,
			},
			"kafka_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"broker": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.NoZeroValues,
						},
						"topic": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  tfdms.KafkaDefaultTopic,
						},
					},
				},
			},
			"kinesis_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"message_format": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      dms.MessageFormatValueJson,
							ValidateFunc: validation.StringInSlice(dms.MessageFormatValue_Values(), false),
						},
						"service_access_role_arn": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validateArn,
						},
						"stream_arn": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validateArn,
						},
					},
				},
			},
			"kms_key_arn": {
				Type:         schema.TypeString,
				Computed:     true,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validateArn,
			},
			"mongodb_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"auth_type": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      dms.AuthTypeValuePassword,
							ValidateFunc: validation.StringInSlice(dms.AuthTypeValue_Values(), false),
						},
						"auth_mechanism": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      tfdms.MongoDbAuthMechanismValueDefault,
							ValidateFunc: validation.StringInSlice(tfdms.MongoDbAuthMechanismValue_Values(), false),
						},
						"nesting_level": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      dms.NestingLevelValueNone,
							ValidateFunc: validation.StringInSlice(dms.NestingLevelValue_Values(), false),
						},
						"extract_doc_id": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "false",
						},
						"docs_to_investigate": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "1000",
						},
						"auth_source": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  tfdms.MongoDbAuthSourceAdmin,
						},
					},
				},
			},
			"password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"port": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"s3_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"service_access_role_arn": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"external_table_definition": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"csv_row_delimiter": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "\\n",
						},
						"csv_delimiter": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  ",",
						},
						"bucket_folder": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"bucket_name": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"compression_type": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      tfdms.S3SettingsCompressionTypeNone,
							ValidateFunc: validation.StringInSlice(tfdms.S3SettingsCompressionType_Values(), false),
						},
						"date_partition_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"data_format": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      dms.DataFormatValueCsv,
							ValidateFunc: validation.StringInSlice(dms.DataFormatValue_Values(), false),
						},
						"parquet_timestamp_in_millisecond": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"parquet_version": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      dms.ParquetVersionValueParquet10,
							ValidateFunc: validation.StringInSlice(dms.ParquetVersionValue_Values(), false),
						},
						"encryption_mode": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      tfdms.S3SettingsEncryptionModeSseS3,
							ValidateFunc: validation.StringInSlice(tfdms.S3SettingsEncryptionMode_Values(), false),
						},
						"server_side_encryption_kms_key_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"server_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"service_access_role": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssl_mode": {
				Type:         schema.TypeString,
				Computed:     true,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(dms.DmsSslModeValue_Values(), false),
			},
			"tags":     tagsSchema(),
			"tags_all": tagsSchemaComputed(),
			"username": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},

		CustomizeDiff: SetTagsDiff,
	}
}

func resourceAwsDmsEndpointCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dmsconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(keyvaluetags.New(d.Get("tags").(map[string]interface{})))

	request := &dms.CreateEndpointInput{
		EndpointIdentifier: aws.String(d.Get("endpoint_id").(string)),
		EndpointType:       aws.String(d.Get("endpoint_type").(string)),
		EngineName:         aws.String(d.Get("engine_name").(string)),
		Tags:               tags.IgnoreAws().DatabasemigrationserviceTags(),
	}

	switch d.Get("engine_name").(string) {
	// if dynamodb then add required params
	case "dynamodb":
		request.DynamoDbSettings = &dms.DynamoDbSettings{
			ServiceAccessRoleArn: aws.String(d.Get("service_access_role").(string)),
		}
	case "elasticsearch":
		request.ElasticsearchSettings = &dms.ElasticsearchSettings{
			ServiceAccessRoleArn:    aws.String(d.Get("elasticsearch_settings.0.service_access_role_arn").(string)),
			EndpointUri:             aws.String(d.Get("elasticsearch_settings.0.endpoint_uri").(string)),
			ErrorRetryDuration:      aws.Int64(int64(d.Get("elasticsearch_settings.0.error_retry_duration").(int))),
			FullLoadErrorPercentage: aws.Int64(int64(d.Get("elasticsearch_settings.0.full_load_error_percentage").(int))),
		}
	case "kafka":
		request.KafkaSettings = &dms.KafkaSettings{
			Broker: aws.String(d.Get("kafka_settings.0.broker").(string)),
			Topic:  aws.String(d.Get("kafka_settings.0.topic").(string)),
		}
	case "kinesis":
		request.KinesisSettings = &dms.KinesisSettings{
			MessageFormat:        aws.String(d.Get("kinesis_settings.0.message_format").(string)),
			ServiceAccessRoleArn: aws.String(d.Get("kinesis_settings.0.service_access_role_arn").(string)),
			StreamArn:            aws.String(d.Get("kinesis_settings.0.stream_arn").(string)),
		}
	case "mongodb":
		request.MongoDbSettings = &dms.MongoDbSettings{
			Username:     aws.String(d.Get("username").(string)),
			Password:     aws.String(d.Get("password").(string)),
			ServerName:   aws.String(d.Get("server_name").(string)),
			Port:         aws.Int64(int64(d.Get("port").(int))),
			DatabaseName: aws.String(d.Get("database_name").(string)),
			KmsKeyId:     aws.String(d.Get("kms_key_arn").(string)),

			AuthType:          aws.String(d.Get("mongodb_settings.0.auth_type").(string)),
			AuthMechanism:     aws.String(d.Get("mongodb_settings.0.auth_mechanism").(string)),
			NestingLevel:      aws.String(d.Get("mongodb_settings.0.nesting_level").(string)),
			ExtractDocId:      aws.String(d.Get("mongodb_settings.0.extract_doc_id").(string)),
			DocsToInvestigate: aws.String(d.Get("mongodb_settings.0.docs_to_investigate").(string)),
			AuthSource:        aws.String(d.Get("mongodb_settings.0.auth_source").(string)),
		}

		// Set connection info in top-level namespace as well
		request.Username = aws.String(d.Get("username").(string))
		request.Password = aws.String(d.Get("password").(string))
		request.ServerName = aws.String(d.Get("server_name").(string))
		request.Port = aws.Int64(int64(d.Get("port").(int)))
		request.DatabaseName = aws.String(d.Get("database_name").(string))
	case "s3":
		request.S3Settings = &dms.S3Settings{
			BucketFolder:                  aws.String(d.Get("s3_settings.0.bucket_folder").(string)),
			BucketName:                    aws.String(d.Get("s3_settings.0.bucket_name").(string)),
			CompressionType:               aws.String(d.Get("s3_settings.0.compression_type").(string)),
			CsvDelimiter:                  aws.String(d.Get("s3_settings.0.csv_delimiter").(string)),
			CsvRowDelimiter:               aws.String(d.Get("s3_settings.0.csv_row_delimiter").(string)),
			DataFormat:                    aws.String(d.Get("s3_settings.0.data_format").(string)),
			DatePartitionEnabled:          aws.Bool(d.Get("s3_settings.0.date_partition_enabled").(bool)),
			EncryptionMode:                aws.String(d.Get("s3_settings.0.encryption_mode").(string)),
			ExternalTableDefinition:       aws.String(d.Get("s3_settings.0.external_table_definition").(string)),
			ParquetTimestampInMillisecond: aws.Bool(d.Get("s3_settings.0.parquet_timestamp_in_millisecond").(bool)),
			ParquetVersion:                aws.String(d.Get("s3_settings.0.parquet_version").(string)),
			ServerSideEncryptionKmsKeyId:  aws.String(d.Get("s3_settings.0.server_side_encryption_kms_key_id").(string)),
			ServiceAccessRoleArn:          aws.String(d.Get("s3_settings.0.service_access_role_arn").(string)),
		}
	default:
		request.Password = aws.String(d.Get("password").(string))
		request.Port = aws.Int64(int64(d.Get("port").(int)))
		request.ServerName = aws.String(d.Get("server_name").(string))
		request.Username = aws.String(d.Get("username").(string))

		if v, ok := d.GetOk("database_name"); ok {
			request.DatabaseName = aws.String(v.(string))
		}

		if v, ok := d.GetOk("kms_key_arn"); ok {
			request.KmsKeyId = aws.String(v.(string))
		}
	}

	if v, ok := d.GetOk("certificate_arn"); ok {
		request.CertificateArn = aws.String(v.(string))
	}

	// Send ExtraConnectionAttributes in the API request for all resource types
	// per https://github.com/hashicorp/terraform-provider-aws/issues/8009
	if v, ok := d.GetOk("extra_connection_attributes"); ok {
		request.ExtraConnectionAttributes = aws.String(v.(string))
	}

	if v, ok := d.GetOk("ssl_mode"); ok {
		request.SslMode = aws.String(v.(string))
	}

	log.Println("[DEBUG] DMS create endpoint:", request)

	err := resource.Retry(5*time.Minute, func() *resource.RetryError {
		_, err := conn.CreateEndpoint(request)
		if isAWSErr(err, "AccessDeniedFault", "") {
			return resource.RetryableError(err)
		}
		if err != nil {
			return resource.NonRetryableError(err)
		}

		// Successful delete
		return nil
	})
	if isResourceTimeoutError(err) {
		_, err = conn.CreateEndpoint(request)
	}
	if err != nil {
		return fmt.Errorf("Error creating DMS endpoint: %s", err)
	}

	d.SetId(d.Get("endpoint_id").(string))
	return resourceAwsDmsEndpointRead(d, meta)
}

func resourceAwsDmsEndpointRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dmsconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*AWSClient).IgnoreTagsConfig

	response, err := conn.DescribeEndpoints(&dms.DescribeEndpointsInput{
		Filters: []*dms.Filter{
			{
				Name:   aws.String("endpoint-id"),
				Values: []*string{aws.String(d.Id())}, // Must use d.Id() to work with import.
			},
		},
	})
	if err != nil {
		if dmserr, ok := err.(awserr.Error); ok && dmserr.Code() == "ResourceNotFoundFault" {
			log.Printf("[DEBUG] DMS Replication Endpoint %q Not Found", d.Id())
			d.SetId("")
			return nil
		}
		return err
	}

	err = resourceAwsDmsEndpointSetState(d, response.Endpoints[0])
	if err != nil {
		return err
	}

	tags, err := keyvaluetags.DatabasemigrationserviceListTags(conn, d.Get("endpoint_arn").(string))

	if err != nil {
		return fmt.Errorf("error listing tags for DMS Endpoint (%s): %s", d.Get("endpoint_arn").(string), err)
	}

	tags = tags.IgnoreAws().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	return nil
}

func resourceAwsDmsEndpointUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dmsconn

	request := &dms.ModifyEndpointInput{
		EndpointArn: aws.String(d.Get("endpoint_arn").(string)),
	}
	hasChanges := false

	if d.HasChange("endpoint_type") {
		request.EndpointType = aws.String(d.Get("endpoint_type").(string))
		hasChanges = true
	}

	if d.HasChange("certificate_arn") {
		request.CertificateArn = aws.String(d.Get("certificate_arn").(string))
		hasChanges = true
	}

	if d.HasChange("service_access_role") {
		request.DynamoDbSettings = &dms.DynamoDbSettings{
			ServiceAccessRoleArn: aws.String(d.Get("service_access_role").(string)),
		}
		hasChanges = true
	}

	if d.HasChange("endpoint_type") {
		request.EndpointType = aws.String(d.Get("endpoint_type").(string))
		hasChanges = true
	}

	if d.HasChange("engine_name") {
		request.EngineName = aws.String(d.Get("engine_name").(string))
		hasChanges = true
	}

	if d.HasChange("extra_connection_attributes") {
		request.ExtraConnectionAttributes = aws.String(d.Get("extra_connection_attributes").(string))
		hasChanges = true
	}

	if d.HasChange("ssl_mode") {
		request.SslMode = aws.String(d.Get("ssl_mode").(string))
		hasChanges = true
	}

	if d.HasChange("tags_all") {
		arn := d.Get("endpoint_arn").(string)
		o, n := d.GetChange("tags_all")

		if err := keyvaluetags.DatabasemigrationserviceUpdateTags(conn, arn, o, n); err != nil {
			return fmt.Errorf("error updating DMS Endpoint (%s) tags: %s", arn, err)
		}
	}

	switch d.Get("engine_name").(string) {
	case "dynamodb":
		if d.HasChange("service_access_role") {
			request.DynamoDbSettings = &dms.DynamoDbSettings{
				ServiceAccessRoleArn: aws.String(d.Get("service_access_role").(string)),
			}
			hasChanges = true
		}
	case "elasticsearch":
		if d.HasChanges(
			"elasticsearch_settings.0.endpoint_uri",
			"elasticsearch_settings.0.error_retry_duration",
			"elasticsearch_settings.0.full_load_error_percentage",
			"elasticsearch_settings.0.service_access_role_arn") {
			request.ElasticsearchSettings = &dms.ElasticsearchSettings{
				ServiceAccessRoleArn:    aws.String(d.Get("elasticsearch_settings.0.service_access_role_arn").(string)),
				EndpointUri:             aws.String(d.Get("elasticsearch_settings.0.endpoint_uri").(string)),
				ErrorRetryDuration:      aws.Int64(int64(d.Get("elasticsearch_settings.0.error_retry_duration").(int))),
				FullLoadErrorPercentage: aws.Int64(int64(d.Get("elasticsearch_settings.0.full_load_error_percentage").(int))),
			}
			request.EngineName = aws.String(d.Get("engine_name").(string))
			hasChanges = true
		}
	case "kafka":
		if d.HasChanges(
			"kafka_settings.0.broker",
			"kafka_settings.0.topic") {
			request.KafkaSettings = &dms.KafkaSettings{
				Broker: aws.String(d.Get("kafka_settings.0.broker").(string)),
				Topic:  aws.String(d.Get("kafka_settings.0.topic").(string)),
			}
			request.EngineName = aws.String(d.Get("engine_name").(string))
			hasChanges = true
		}
	case "kinesis":
		if d.HasChanges(
			"kinesis_settings.0.service_access_role_arn",
			"kinesis_settings.0.stream_arn") {
			// Intentionally omitting MessageFormat, because it's rejected on ModifyEndpoint calls.
			// "An error occurred (InvalidParameterValueException) when calling the ModifyEndpoint
			// operation: Message format  cannot be modified for kinesis endpoints."
			request.KinesisSettings = &dms.KinesisSettings{
				ServiceAccessRoleArn: aws.String(d.Get("kinesis_settings.0.service_access_role_arn").(string)),
				StreamArn:            aws.String(d.Get("kinesis_settings.0.stream_arn").(string)),
			}
			request.EngineName = aws.String(d.Get("engine_name").(string)) // Must be included (should be 'kinesis')
			hasChanges = true
		}
	case "mongodb":
		if d.HasChanges(
			"username", "password", "server_name", "port", "database_name", "mongodb_settings.0.auth_type",
			"mongodb_settings.0.auth_mechanism", "mongodb_settings.0.nesting_level", "mongodb_settings.0.extract_doc_id",
			"mongodb_settings.0.docs_to_investigate", "mongodb_settings.0.auth_source") {
			request.MongoDbSettings = &dms.MongoDbSettings{
				Username:     aws.String(d.Get("username").(string)),
				Password:     aws.String(d.Get("password").(string)),
				ServerName:   aws.String(d.Get("server_name").(string)),
				Port:         aws.Int64(int64(d.Get("port").(int))),
				DatabaseName: aws.String(d.Get("database_name").(string)),
				KmsKeyId:     aws.String(d.Get("kms_key_arn").(string)),

				AuthType:          aws.String(d.Get("mongodb_settings.0.auth_type").(string)),
				AuthMechanism:     aws.String(d.Get("mongodb_settings.0.auth_mechanism").(string)),
				NestingLevel:      aws.String(d.Get("mongodb_settings.0.nesting_level").(string)),
				ExtractDocId:      aws.String(d.Get("mongodb_settings.0.extract_doc_id").(string)),
				DocsToInvestigate: aws.String(d.Get("mongodb_settings.0.docs_to_investigate").(string)),
				AuthSource:        aws.String(d.Get("mongodb_settings.0.auth_source").(string)),
			}
			request.EngineName = aws.String(d.Get("engine_name").(string)) // Must be included (should be 'mongodb')

			// Update connection info in top-level namespace as well
			request.Username = aws.String(d.Get("username").(string))
			request.Password = aws.String(d.Get("password").(string))
			request.ServerName = aws.String(d.Get("server_name").(string))
			request.Port = aws.Int64(int64(d.Get("port").(int)))
			request.DatabaseName = aws.String(d.Get("database_name").(string))

			hasChanges = true
		}
	case "s3":
		if d.HasChanges(
			"s3_settings.0.service_access_role_arn", "s3_settings.0.external_table_definition",
			"s3_settings.0.csv_row_delimiter", "s3_settings.0.csv_delimiter", "s3_settings.0.bucket_folder",
			"s3_settings.0.bucket_name", "s3_settings.0.compression_type", "s3_settings.0.data_format",
			"s3_settings.0.parquet_version", "s3_settings.0.parquet_timestamp_in_millisecond",
			"s3_settings.0.encryption_mode", "s3_settings.0.server_side_encryption_kms_key_id") {
			request.S3Settings = &dms.S3Settings{
				BucketFolder:                  aws.String(d.Get("s3_settings.0.bucket_folder").(string)),
				BucketName:                    aws.String(d.Get("s3_settings.0.bucket_name").(string)),
				CompressionType:               aws.String(d.Get("s3_settings.0.compression_type").(string)),
				CsvDelimiter:                  aws.String(d.Get("s3_settings.0.csv_delimiter").(string)),
				CsvRowDelimiter:               aws.String(d.Get("s3_settings.0.csv_row_delimiter").(string)),
				DataFormat:                    aws.String(d.Get("s3_settings.0.data_format").(string)),
				EncryptionMode:                aws.String(d.Get("s3_settings.0.encryption_mode").(string)),
				ExternalTableDefinition:       aws.String(d.Get("s3_settings.0.external_table_definition").(string)),
				ParquetTimestampInMillisecond: aws.Bool(d.Get("s3_settings.0.parquet_timestamp_in_millisecond").(bool)),
				ParquetVersion:                aws.String(d.Get("s3_settings.0.parquet_version").(string)),
				ServerSideEncryptionKmsKeyId:  aws.String(d.Get("s3_settings.0.server_side_encryption_kms_key_id").(string)),
				ServiceAccessRoleArn:          aws.String(d.Get("s3_settings.0.service_access_role_arn").(string)),
			}
			request.EngineName = aws.String(d.Get("engine_name").(string)) // Must be included (should be 's3')
			hasChanges = true
		}
	default:
		if d.HasChange("database_name") {
			request.DatabaseName = aws.String(d.Get("database_name").(string))
			hasChanges = true
		}

		if d.HasChange("password") {
			request.Password = aws.String(d.Get("password").(string))
			hasChanges = true
		}

		if d.HasChange("port") {
			request.Port = aws.Int64(int64(d.Get("port").(int)))
			hasChanges = true
		}

		if d.HasChange("server_name") {
			request.ServerName = aws.String(d.Get("server_name").(string))
			hasChanges = true
		}

		if d.HasChange("username") {
			request.Username = aws.String(d.Get("username").(string))
			hasChanges = true
		}
	}

	if hasChanges {
		log.Println("[DEBUG] DMS update endpoint:", request)

		_, err := conn.ModifyEndpoint(request)
		if err != nil {
			return err
		}

		return resourceAwsDmsEndpointRead(d, meta)
	}

	return nil
}

func resourceAwsDmsEndpointDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dmsconn

	request := &dms.DeleteEndpointInput{
		EndpointArn: aws.String(d.Get("endpoint_arn").(string)),
	}

	log.Printf("[DEBUG] DMS delete endpoint: %#v", request)

	_, err := conn.DeleteEndpoint(request)
	return err
}

func resourceAwsDmsEndpointSetState(d *schema.ResourceData, endpoint *dms.Endpoint) error {
	d.SetId(aws.StringValue(endpoint.EndpointIdentifier))

	d.Set("certificate_arn", endpoint.CertificateArn)
	d.Set("endpoint_arn", endpoint.EndpointArn)
	d.Set("endpoint_id", endpoint.EndpointIdentifier)
	// For some reason the AWS API only accepts lowercase type but returns it as uppercase
	d.Set("endpoint_type", strings.ToLower(*endpoint.EndpointType))
	d.Set("engine_name", endpoint.EngineName)
	d.Set("extra_connection_attributes", endpoint.ExtraConnectionAttributes)

	switch *endpoint.EngineName {
	case "dynamodb":
		if endpoint.DynamoDbSettings != nil {
			d.Set("service_access_role", endpoint.DynamoDbSettings.ServiceAccessRoleArn)
		} else {
			d.Set("service_access_role", "")
		}
	case "elasticsearch":
		if err := d.Set("elasticsearch_settings", flattenDmsElasticsearchSettings(endpoint.ElasticsearchSettings)); err != nil {
			return fmt.Errorf("Error setting elasticsearch for DMS: %s", err)
		}
	case "kafka":
		if err := d.Set("kafka_settings", flattenDmsKafkaSettings(endpoint.KafkaSettings)); err != nil {
			return fmt.Errorf("Error setting kafka_settings for DMS: %s", err)
		}
	case "kinesis":
		if err := d.Set("kinesis_settings", flattenDmsKinesisSettings(endpoint.KinesisSettings)); err != nil {
			return fmt.Errorf("Error setting kinesis_settings for DMS: %s", err)
		}
	case "mongodb":
		if endpoint.MongoDbSettings != nil {
			d.Set("username", endpoint.MongoDbSettings.Username)
			d.Set("server_name", endpoint.MongoDbSettings.ServerName)
			d.Set("port", endpoint.MongoDbSettings.Port)
			d.Set("database_name", endpoint.MongoDbSettings.DatabaseName)
		} else {
			d.Set("username", endpoint.Username)
			d.Set("server_name", endpoint.ServerName)
			d.Set("port", endpoint.Port)
			d.Set("database_name", endpoint.DatabaseName)
		}
		if err := d.Set("mongodb_settings", flattenDmsMongoDbSettings(endpoint.MongoDbSettings)); err != nil {
			return fmt.Errorf("Error setting mongodb_settings for DMS: %s", err)
		}
	case "s3":
		if err := d.Set("s3_settings", flattenDmsS3Settings(endpoint.S3Settings)); err != nil {
			return fmt.Errorf("Error setting s3_settings for DMS: %s", err)
		}
	default:
		d.Set("database_name", endpoint.DatabaseName)
		d.Set("extra_connection_attributes", endpoint.ExtraConnectionAttributes)
		d.Set("port", endpoint.Port)
		d.Set("server_name", endpoint.ServerName)
		d.Set("username", endpoint.Username)
	}

	d.Set("kms_key_arn", endpoint.KmsKeyId)
	d.Set("ssl_mode", endpoint.SslMode)

	return nil
}

func flattenDmsElasticsearchSettings(settings *dms.ElasticsearchSettings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	m := map[string]interface{}{
		"endpoint_uri":               aws.StringValue(settings.EndpointUri),
		"error_retry_duration":       aws.Int64Value(settings.ErrorRetryDuration),
		"full_load_error_percentage": aws.Int64Value(settings.FullLoadErrorPercentage),
		"service_access_role_arn":    aws.StringValue(settings.ServiceAccessRoleArn),
	}

	return []map[string]interface{}{m}
}

func flattenDmsKafkaSettings(settings *dms.KafkaSettings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	m := map[string]interface{}{
		"broker": aws.StringValue(settings.Broker),
		"topic":  aws.StringValue(settings.Topic),
	}

	return []map[string]interface{}{m}
}

func flattenDmsKinesisSettings(settings *dms.KinesisSettings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	m := map[string]interface{}{
		"message_format":          aws.StringValue(settings.MessageFormat),
		"service_access_role_arn": aws.StringValue(settings.ServiceAccessRoleArn),
		"stream_arn":              aws.StringValue(settings.StreamArn),
	}

	return []map[string]interface{}{m}
}

func flattenDmsMongoDbSettings(settings *dms.MongoDbSettings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	m := map[string]interface{}{
		"auth_type":           aws.StringValue(settings.AuthType),
		"auth_mechanism":      aws.StringValue(settings.AuthMechanism),
		"nesting_level":       aws.StringValue(settings.NestingLevel),
		"extract_doc_id":      aws.StringValue(settings.ExtractDocId),
		"docs_to_investigate": aws.StringValue(settings.DocsToInvestigate),
		"auth_source":         aws.StringValue(settings.AuthSource),
	}

	return []map[string]interface{}{m}
}

func flattenDmsS3Settings(settings *dms.S3Settings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	m := map[string]interface{}{
		"bucket_folder":                     aws.StringValue(settings.BucketFolder),
		"bucket_name":                       aws.StringValue(settings.BucketName),
		"compression_type":                  aws.StringValue(settings.CompressionType),
		"csv_delimiter":                     aws.StringValue(settings.CsvDelimiter),
		"csv_row_delimiter":                 aws.StringValue(settings.CsvRowDelimiter),
		"data_format":                       aws.StringValue(settings.DataFormat),
		"date_partition_enabled":            aws.BoolValue(settings.DatePartitionEnabled),
		"encryption_mode":                   aws.StringValue(settings.EncryptionMode),
		"external_table_definition":         aws.StringValue(settings.ExternalTableDefinition),
		"parquet_timestamp_in_millisecond":  aws.BoolValue(settings.ParquetTimestampInMillisecond),
		"parquet_version":                   aws.StringValue(settings.ParquetVersion),
		"server_side_encryption_kms_key_id": aws.StringValue(settings.ServerSideEncryptionKmsKeyId),
		"service_access_role_arn":           aws.StringValue(settings.ServiceAccessRoleArn),
	}

	return []map[string]interface{}{m}
}

func suppressExtraConnectionAttributesDiffs(_, old, new string, d *schema.ResourceData) bool {
	if d.Id() != "" {
		o := extraConnectionAttributesToSet(old)
		n := extraConnectionAttributesToSet(new)

		var config *schema.Set
		// when the engine is "s3" or "mongodb", the extra_connection_attributes
		// can consist of a subset of the attributes configured in the {engine}_settings block;
		// fields such as service_access_role_arn (in the case of "s3") are not returned from the API in
		// extra_connection_attributes thus we take the Set difference to ensure
		// the returned attributes were set in the {engine}_settings block or originally
		// in the extra_connection_attributes field
		if v, ok := d.GetOk("mongodb_settings"); ok {
			config = engineSettingsToSet(v.([]interface{}))
		} else if v, ok := d.GetOk("s3_settings"); ok {
			config = engineSettingsToSet(v.([]interface{}))
		}

		if o != nil && config != nil {
			diff := o.Difference(config)

			return diff.Len() == 0 || diff.Equal(n)
		}
	}
	return false
}

// extraConnectionAttributesToSet accepts an extra_connection_attributes
// string in the form of "key=value;key2=value2;" and returns
// the Set representation, with each element being the key/value pair
func extraConnectionAttributesToSet(extra string) *schema.Set {
	if extra == "" {
		return nil
	}

	s := &schema.Set{F: schema.HashString}

	parts := strings.Split(extra, ";")
	for _, part := range parts {
		kvParts := strings.Split(part, "=")
		if len(kvParts) != 2 {
			continue
		}

		k, v := kvParts[0], kvParts[1]
		// normalize key, from camelCase to snake_case,
		// and value where hyphens maybe used in a config
		// but the API returns with underscores
		matchAllCap := regexp.MustCompile("([a-z])([A-Z])")
		key := matchAllCap.ReplaceAllString(k, "${1}_${2}")
		normalizedVal := strings.Replace(strings.ToLower(v), "-", "_", -1)

		s.Add(fmt.Sprintf("%s=%s", strings.ToLower(key), normalizedVal))
	}

	return s
}

// engineSettingsToSet accepts the {engine}_settings block as a list
// and returns the Set representation, with each element being the key/value pair
func engineSettingsToSet(l []interface{}) *schema.Set {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	tfMap, ok := l[0].(map[string]interface{})
	if !ok {
		return nil
	}

	s := &schema.Set{F: schema.HashString}

	for k, v := range tfMap {
		switch t := v.(type) {
		case string:
			// normalize value for changes in case or where hyphens
			// maybe used in a config but the API returns with underscores
			normalizedVal := strings.Replace(strings.ToLower(t), "-", "_", -1)
			s.Add(fmt.Sprintf("%s=%v", k, normalizedVal))
		default:
			s.Add(fmt.Sprintf("%s=%v", k, t))
		}
	}

	return s
}
