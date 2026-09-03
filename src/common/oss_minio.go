package common

import (
	"bigdevops/src/config"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// InitMinioClient 初始化 MinIO 客户端并确保存储桶已创建
func InitMinioClient(sc *config.ServerConfig) (*minio.Client, error) {
	if sc.OSSC == nil || !sc.OSSC.Enable {
		return nil, fmt.Errorf("MinIO OSS 未启用，请在配置文件 server.yml 中配置 oss 部分")
	}

	minioClient, err := minio.New(sc.OSSC.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(sc.OSSC.AccessKey, sc.OSSC.SecretKey, ""),
		Secure: sc.OSSC.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查存储桶是否存在
	exists, errBucketExists := minioClient.BucketExists(ctx, sc.OSSC.BucketName)
	if errBucketExists == nil && !exists {
		// 自动创建存储桶
		err = minioClient.MakeBucket(ctx, sc.OSSC.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			sc.Logger.Error("[MinIO] 自动创建存储桶失败", zap.String("bucket", sc.OSSC.BucketName), zap.Error(err))
			return nil, fmt.Errorf("创建存储桶失败: %w", err)
		}
		sc.Logger.Info("[MinIO] 自动创建存储桶成功", zap.String("bucket", sc.OSSC.BucketName))

		// 设置存储桶只读公开策略（方便图片公共访问）
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/*"]
				}
			]
		}`, sc.OSSC.BucketName)
		_ = minioClient.SetBucketPolicy(ctx, sc.OSSC.BucketName, policy)
	}

	return minioClient, nil
}

// UploadAvatarToMinio 上传用户头像到 MinIO，返回公共可访问的图片 URL
func UploadAvatarToMinio(sc *config.ServerConfig, objectName string, reader io.Reader, objectSize int64, contentType string) (string, error) {
	client, err := InitMinioClient(sc)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if contentType == "" {
		contentType = "image/png"
	}

	uploadInfo, err := client.PutObject(ctx, sc.OSSC.BucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		sc.Logger.Error("[MinIO] 上传头像文件失败", zap.String("object", objectName), zap.Error(err))
		return "", fmt.Errorf("上传文件到 MinIO 失败: %w", err)
	}

	sc.Logger.Info("[MinIO] 上传头像成功", zap.String("object", uploadInfo.Key), zap.Int64("size", uploadInfo.Size))

	// 拼接访问 URL
	urlPrefix := strings.TrimRight(sc.OSSC.UrlPrefix, "/")
	if urlPrefix == "" {
		scheme := "http"
		if sc.OSSC.UseSSL {
			scheme = "https"
		}
		urlPrefix = fmt.Sprintf("%s://%s/%s", scheme, sc.OSSC.Endpoint, sc.OSSC.BucketName)
	}

	fileURL := fmt.Sprintf("%s/%s", urlPrefix, strings.TrimLeft(objectName, "/"))
	return fileURL, nil
}
