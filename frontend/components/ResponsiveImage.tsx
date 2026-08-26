import * as preact from "preact";
import "./responsive-image-styles";

interface ResponsiveImageProps {
  photoId: number;
  alt: string;
  className?: string;
  sizes?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
  onClick?: () => void;
  width?: number;
  height?: number;
  status?: number;
}

export const ResponsiveImage = ({
  photoId,
  alt,
  className,
  sizes = "100vw",
  loading = "lazy",
  fetchpriority = "auto",
  onClick,
  width,
  height,
  status = 0,
}: ResponsiveImageProps) => {
  const imageSizes = [
    { size: "thumb", width: 300 },
    { size: "medium", width: 600 },
    { size: "large", width: 900 },
    { size: "xlarge", width: 1200 },
  ];

  const srcset = imageSizes
    .map(img => `/api/photo/${photoId}/${img.size} ${img.width}w`)
    .join(", ");

  const src = `/api/photo/${photoId}/medium`;

  const wrapperClass = status === 1 ? "processing-image-wrapper" : undefined;
  const imageClass = status === 1 ? `${className || ""} processing-image`.trim() : className;

  const imageElement = (
    <picture>
      <source srcSet={srcset} sizes={sizes} type="image/avif" />
      <source srcSet={srcset} sizes={sizes} type="image/webp" />

      <img
        src={src}
        alt={alt}
        className={imageClass}
        loading={loading}
        fetchpriority={fetchpriority}
        onClick={onClick}
        width={width}
        height={height}
        srcSet={srcset}
        sizes={sizes}
      />
    </picture>
  );

  if (status === 1) {
    return (
      <div className={wrapperClass}>
        {imageElement}
        <div className="processing-overlay">
          <div className="processing-spinner"></div>
          <div className="processing-text">Processing...</div>
        </div>
      </div>
    );
  }

  return imageElement;
};

interface ThumbnailImageProps {
  photoId: number;
  alt: string;
  className?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
  onClick?: () => void;
  status?: number;
}

export const ThumbnailImage = ({
  photoId,
  alt,
  className,
  loading = "lazy",
  fetchpriority = "auto",
  onClick,
  status,
}: ThumbnailImageProps) => {
  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      onClick={onClick}
      status={status}
      sizes="(max-width: 480px) 150px, (max-width: 768px) 200px, 300px"
      width={300}
      height={300}
    />
  );
};

interface ProfileImageProps {
  photoId: number;
  alt: string;
  className?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
  status?: number;
  cropX?: number;
  cropY?: number;
  cropScale?: number;
}

export const ProfileImage = ({
  photoId,
  alt,
  className,
  loading = "lazy",
  fetchpriority = "auto",
  status,
  cropX = 50,
  cropY = 50,
  cropScale = 1,
}: ProfileImageProps) => {
  const effectiveCropScale = cropScale || 1;
  const effectiveCropX = cropX || 50;
  const effectiveCropY = cropY || 50;
  const hasCrop = effectiveCropScale > 1 || effectiveCropX !== 50 || effectiveCropY !== 50;

  if (hasCrop) {
    const imageStyle = {
      transform: `scale(${effectiveCropScale})`,
      transformOrigin: `${effectiveCropX}% ${effectiveCropY}%`,
    };

    return (
      <div className={`profile-image-crop-container ${className || ""}`}>
        <div className="profile-image-inner" style={imageStyle}>
          <ResponsiveImage
            photoId={photoId}
            alt={alt}
            className="profile-image-cropped"
            loading={loading}
            fetchpriority={fetchpriority}
            status={status}
            sizes="(max-width: 480px) 300px, (max-width: 768px) 600px, 900px"
            width={600}
            height={600}
          />
        </div>
      </div>
    );
  }

  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      status={status}
      sizes="(max-width: 480px) 300px, (max-width: 768px) 600px, 900px"
      width={600}
      height={600}
    />
  );
};

interface FullImageProps {
  photoId: number;
  alt: string;
  className?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
  status?: number;
}

export const FullImage = ({
  photoId,
  alt,
  className,
  loading = "eager",
  fetchpriority = "high",
  status,
}: FullImageProps) => {
  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      status={status}
      sizes="(max-width: 768px) 100vw, (max-width: 1200px) 80vw, 1200px"
    />
  );
};
