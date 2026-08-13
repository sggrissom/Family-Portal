import * as preact from "preact";
import { useRef, useState, useEffect } from "preact/hooks";
import { ProfileImage } from "./ResponsiveImage";
import "./crop-selector-styles";

export interface CropValues {
  cropX: number; // 0-100
  cropY: number; // 0-100
  cropScale: number; // 1.0+
}

interface CropSelectorProps {
  photoId: number;
  initialCropX?: number;
  initialCropY?: number;
  initialCropScale?: number;
  onCropChange: (values: CropValues) => void;
  onSave: () => void;
  onCancel: () => void;
}

export const CropSelector = ({
  photoId,
  initialCropX = 50,
  initialCropY = 50,
  initialCropScale = 1,
  onCropChange,
  onSave,
  onCancel,
}: CropSelectorProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [cropX, setCropX] = useState(initialCropX);
  const [cropY, setCropY] = useState(initialCropY);
  const [cropScale, setCropScale] = useState(initialCropScale);
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  const [startCrop, setStartCrop] = useState({ x: initialCropX, y: initialCropY });

  // Image src for cropping (use medium size for performance)
  const imageSrc = `/api/photo/${photoId}/large`;

  // Notify parent of crop changes
  useEffect(() => {
    onCropChange({ cropX, cropY, cropScale });
  }, [cropX, cropY, cropScale]);

  const handlePointerDown = (e: PointerEvent) => {
    e.preventDefault();
    (e.currentTarget as HTMLDivElement).setPointerCapture(e.pointerId);
    setIsDragging(true);
    setDragStart({ x: e.clientX, y: e.clientY });
    setStartCrop({ x: cropX, y: cropY });
  };

  const handlePointerMove = (e: PointerEvent) => {
    if (!isDragging || !containerRef.current) return;

    e.preventDefault();

    const rect = containerRef.current.getBoundingClientRect();
    const deltaX = e.clientX - dragStart.x;
    const deltaY = e.clientY - dragStart.y;

    // Convert pixel delta to percentage (invert because dragging moves the viewport)
    // Higher scale = more sensitive dragging
    const sensitivity = 100 / cropScale;
    const newX = Math.max(0, Math.min(100, startCrop.x - (deltaX / rect.width) * sensitivity));
    const newY = Math.max(0, Math.min(100, startCrop.y - (deltaY / rect.height) * sensitivity));

    setCropX(newX);
    setCropY(newY);
  };

  const handlePointerUp = (e: PointerEvent) => {
    const container = e.currentTarget as HTMLDivElement;
    if (container.hasPointerCapture(e.pointerId)) {
      container.releasePointerCapture(e.pointerId);
    }
    setIsDragging(false);
  };

  // Handle wheel zoom
  const handleWheel = (e: WheelEvent) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? -0.1 : 0.1;
    setCropScale(prev => Math.max(1, Math.min(3, prev + delta)));
  };

  // Slider zoom control
  const handleScaleChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    setCropScale(parseFloat(target.value));
  };

  // Reset to defaults
  const handleReset = () => {
    setCropX(50);
    setCropY(50);
    setCropScale(1);
  };

  // Keep the page behind the editor fixed, particularly while manipulating the
  // crop or zoom slider on touch devices.
  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, []);

  const previewStyle = {
    transform: `scale(${cropScale})`,
    transformOrigin: `${cropX}% ${cropY}%`,
  };

  return (
    <div className="crop-selector-modal">
      <div className="crop-selector-content">
        <div className="crop-selector-header">
          <h2>Adjust Profile Photo</h2>
          <p>Drag to pan, use slider to zoom</p>
        </div>

        <div className="crop-selector-body">
          {/* Main crop area */}
          <div
            ref={containerRef}
            className={`crop-container ${isDragging ? "dragging" : ""}`}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
            onWheel={handleWheel}
          >
            <div className="crop-image-wrapper" style={previewStyle}>
              <img src={imageSrc} alt="Crop preview" className="crop-image" draggable={false} />
            </div>
            <div className="crop-overlay">
              <div className="crop-circle"></div>
            </div>
          </div>

          {/* Preview of result */}
          <div className="crop-preview-section">
            <h4>Preview</h4>
            <div className="crop-preview-container">
              <ProfileImage
                photoId={photoId}
                alt="Profile photo preview"
                cropX={cropX}
                cropY={cropY}
                cropScale={cropScale}
              />
            </div>
          </div>
        </div>

        {/* Zoom slider */}
        <div className="crop-controls">
          <label className="zoom-label">
            <span>Zoom: {cropScale.toFixed(1)}x</span>
            <input
              type="range"
              min="1"
              max="3"
              step="0.1"
              value={cropScale}
              onInput={handleScaleChange}
              className="zoom-slider"
            />
          </label>
          <button type="button" className="btn btn-outline btn-sm" onClick={handleReset}>
            Reset
          </button>
        </div>

        {/* Action buttons */}
        <div className="crop-selector-actions">
          <button type="button" className="btn btn-outline" onClick={onCancel}>
            Cancel
          </button>
          <button type="button" className="btn btn-primary" onClick={onSave}>
            Save as Profile Photo
          </button>
        </div>
      </div>
    </div>
  );
};
