import * as preact from "preact";
import { useRef, useState, useEffect } from "preact/hooks";
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

  const handleMouseDown = (e: MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
    setDragStart({ x: e.clientX, y: e.clientY });
    setStartCrop({ x: cropX, y: cropY });
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!isDragging || !containerRef.current) return;

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

  const handleMouseUp = () => {
    setIsDragging(false);
  };

  // Touch support
  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length === 1) {
      const touch = e.touches[0];
      setIsDragging(true);
      setDragStart({ x: touch.clientX, y: touch.clientY });
      setStartCrop({ x: cropX, y: cropY });
    }
  };

  const handleTouchMove = (e: TouchEvent) => {
    if (!isDragging || e.touches.length !== 1 || !containerRef.current) return;

    const touch = e.touches[0];
    const rect = containerRef.current.getBoundingClientRect();
    const deltaX = touch.clientX - dragStart.x;
    const deltaY = touch.clientY - dragStart.y;

    const sensitivity = 100 / cropScale;
    const newX = Math.max(0, Math.min(100, startCrop.x - (deltaX / rect.width) * sensitivity));
    const newY = Math.max(0, Math.min(100, startCrop.y - (deltaY / rect.height) * sensitivity));

    setCropX(newX);
    setCropY(newY);
  };

  const handleTouchEnd = () => {
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

  // Setup global mouse events
  useEffect(() => {
    if (isDragging) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
      document.addEventListener("touchmove", handleTouchMove);
      document.addEventListener("touchend", handleTouchEnd);
      return () => {
        document.removeEventListener("mousemove", handleMouseMove);
        document.removeEventListener("mouseup", handleMouseUp);
        document.removeEventListener("touchmove", handleTouchMove);
        document.removeEventListener("touchend", handleTouchEnd);
      };
    }
  }, [isDragging, dragStart, startCrop, cropScale]);

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
            onMouseDown={handleMouseDown}
            onTouchStart={handleTouchStart}
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
              <div className="crop-preview-image-wrapper" style={previewStyle}>
                <img
                  src={imageSrc}
                  alt="Preview"
                  className="crop-preview-image"
                  draggable={false}
                />
              </div>
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
