import * as preact from "preact";

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
    .map((img) => `/api/photo/${photoId}/${img.size} ${img.width}w`)
    .join(", ");

  // Default src (fallback)
  const src = `/api/photo/${photoId}/medium`;

  return (
    <picture>
      {/* Modern formats with srcset */}
      <source
        srcSet={srcset}
        sizes={sizes}
        type="image/avif"
      />
      <source
        srcSet={srcset}
        sizes={sizes}
        type="image/webp"
      />

      {/* Fallback img element */}
      <img
        src={src}
        alt={alt}
        className={className}
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
};

interface ThumbnailImageProps {
  photoId: number;
  alt: string;
  className?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
  onClick?: () => void;
}

export const ThumbnailImage = ({
  photoId,
  alt,
  className,
  loading = "lazy",
  fetchpriority = "auto",
  onClick,
}: ThumbnailImageProps) => {
  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      onClick={onClick}
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
}

export const ProfileImage = ({
  photoId,
  alt,
  className,
  loading = "lazy",
  fetchpriority = "auto",
}: ProfileImageProps) => {
  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      sizes="(max-width: 480px) 80px, (max-width: 768px) 120px, 150px"
      width={150}
      height={150}
    />
  );
};

interface FullImageProps {
  photoId: number;
  alt: string;
  className?: string;
  loading?: "lazy" | "eager";
  fetchpriority?: "high" | "low" | "auto";
}

export const FullImage = ({
  photoId,
  alt,
  className,
  loading = "eager",
  fetchpriority = "high", // Default to high priority for full images
}: FullImageProps) => {
  return (
    <ResponsiveImage
      photoId={photoId}
      alt={alt}
      className={className}
      loading={loading}
      fetchpriority={fetchpriority}
      sizes="(max-width: 768px) 100vw, (max-width: 1200px) 80vw, 1200px"
    />
  );
};