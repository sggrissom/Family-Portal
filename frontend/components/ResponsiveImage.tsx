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
  status?: number; // 0 = active, 1 = processing, 2 = failed
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
  // Define responsive breakpoints and corresponding image sizes
  const imageSizes = [
    { size: "small", width: 150 },
    { size: "thumb", width: 300 },
    { size: "medium", width: 600 },
    { size: "large", width: 900 },
    { size: "xlarge", width: 1200 },
    { size: "xxlarge", width: 1800 },
  ];

  // Generate srcset for different sizes
  const srcset = imageSizes
    .map(img => `/api/photo/${photoId}/${img.size} ${img.width}w`)
    .join(", ");

  // Default src (fallback)
  const src = `/api/photo/${photoId}/medium`;

  // Add processing wrapper class if needed
  const wrapperClass = status === 1 ? "processing-image-wrapper" : undefined;
  const imageClass = status === 1 ? `${className || ""} processing-image`.trim() : className;

  const imageElement = (
    <picture>
      {/* Modern formats with srcset */}
      <source srcSet={srcset} sizes={sizes} type="image/avif" />
      <source srcSet={srcset} sizes={sizes} type="image/webp" />

      {/* Fallback img element */}
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

  // Wrap with processing indicator if needed
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
}

export const ProfileImage = ({
  photoId,
  alt,
  className,
  loading = "lazy",
  fetchpriority = "auto",
  status,
}: ProfileImageProps) => {
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
  fetchpriority = "high", // Default to high priority for full images
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
