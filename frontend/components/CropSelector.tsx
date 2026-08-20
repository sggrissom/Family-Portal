import * as preact from "preact";
import { useRef, useState, useEffect } from "preact/hooks";
import { ProfileImage } from "./ResponsiveImage";
import { useModalDialog } from "../hooks/useModalDialog";
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
  const dialogRef = useModalDialog(onCancel);
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

  // Pan and zoom from the keyboard. Dragging was the only way to move the crop,
  // which left the editor unusable without a pointer. The step matches what a
  // small drag does: less at high zoom, where the same pixel covers less image.
  const handleKeyDown = (e: KeyboardEvent) => {
    const step = 5 / cropScale;
    switch (e.key) {
      case "ArrowLeft":
        setCropX(prev => Math.max(0, prev - step));
        break;
      case "ArrowRight":
        setCropX(prev => Math.min(100, prev + step));
        break;
      case "ArrowUp":
        setCropY(prev => Math.max(0, prev - step));
        break;
      case "ArrowDown":
        setCropY(prev => Math.min(100, prev + step));
        break;
      case "+":
      case "=":
        setCropScale(prev => Math.min(3, prev + 0.1));
        break;
      case "-":
        setCropScale(prev => Math.max(1, prev - 0.1));
        break;
      default:
        return;
    }
    e.preventDefault();
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
    <div
      className="crop-selector-modal"
      ref={dialogRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby="cropDialogTitle"
      aria-describedby="cropDialogHint"
    >
      <div className="crop-selector-content">
        <div className="crop-selector-header">
          <h2 id="cropDialogTitle">Adjust Profile Photo</h2>
          <p id="cropDialogHint">
            Drag to pan or use the arrow keys, and the slider to zoom. Escape closes without saving.
          </p>
        </div>

        <div className="crop-selector-body">
          {/* Main crop area */}
          <div
            ref={containerRef}
            className={`crop-container ${isDragging ? "dragging" : ""}`}
            role="application"
            aria-label="Crop area — arrow keys pan the photo"
            tabIndex={0}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
            onWheel={handleWheel}
            onKeyDown={handleKeyDown}
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
