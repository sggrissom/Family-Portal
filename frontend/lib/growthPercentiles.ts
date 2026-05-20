// WHO 2006 Child Growth Standards (0-24 months) and CDC NCHS 2000 Growth Charts (2-20 years)
// Percentiles: 3rd, 15th, 50th, 85th, 97th
// Height in cm, Weight in kg

export interface PercentileRow {
  month: number;
  p3: number;
  p15: number;
  p50: number;
  p85: number;
  p97: number;
}

// WHO Boys Length-for-age (cm), 0-24 months
export const whoHeightBoys: PercentileRow[] = [
  { month: 0, p3: 46.1, p15: 47.9, p50: 49.9, p85: 51.9, p97: 53.7 },
  { month: 1, p3: 50.8, p15: 52.8, p50: 54.7, p85: 56.7, p97: 58.6 },
  { month: 2, p3: 54.4, p15: 56.4, p50: 58.4, p85: 60.4, p97: 62.4 },
  { month: 3, p3: 57.3, p15: 59.4, p50: 61.4, p85: 63.5, p97: 65.5 },
  { month: 4, p3: 59.7, p15: 61.8, p50: 63.9, p85: 66.0, p97: 68.0 },
  { month: 5, p3: 61.7, p15: 63.8, p50: 65.9, p85: 68.0, p97: 70.1 },
  { month: 6, p3: 63.3, p15: 65.5, p50: 67.6, p85: 69.8, p97: 71.9 },
  { month: 7, p3: 64.8, p15: 67.0, p50: 69.2, p85: 71.4, p97: 73.5 },
  { month: 8, p3: 66.2, p15: 68.4, p50: 70.6, p85: 72.9, p97: 75.0 },
  { month: 9, p3: 67.5, p15: 69.7, p50: 72.0, p85: 74.3, p97: 76.5 },
  { month: 10, p3: 68.7, p15: 71.0, p50: 73.3, p85: 75.6, p97: 77.9 },
  { month: 11, p3: 69.9, p15: 72.2, p50: 74.5, p85: 76.9, p97: 79.2 },
  { month: 12, p3: 71.0, p15: 73.4, p50: 75.7, p85: 78.1, p97: 80.5 },
  { month: 13, p3: 72.1, p15: 74.5, p50: 76.9, p85: 79.3, p97: 81.8 },
  { month: 14, p3: 73.1, p15: 75.6, p50: 78.0, p85: 80.5, p97: 83.0 },
  { month: 15, p3: 74.1, p15: 76.6, p50: 79.1, p85: 81.7, p97: 84.2 },
  { month: 16, p3: 75.0, p15: 77.6, p50: 80.2, p85: 82.8, p97: 85.4 },
  { month: 17, p3: 75.9, p15: 78.6, p50: 81.2, p85: 83.9, p97: 86.5 },
  { month: 18, p3: 76.9, p15: 79.6, p50: 82.3, p85: 85.0, p97: 87.7 },
  { month: 19, p3: 77.7, p15: 80.5, p50: 83.2, p85: 86.0, p97: 88.8 },
  { month: 20, p3: 78.6, p15: 81.4, p50: 84.2, p85: 87.0, p97: 89.9 },
  { month: 21, p3: 79.4, p15: 82.3, p50: 85.1, p85: 88.0, p97: 90.9 },
  { month: 22, p3: 80.2, p15: 83.1, p50: 86.0, p85: 88.9, p97: 91.9 },
  { month: 23, p3: 81.0, p15: 83.9, p50: 86.9, p85: 89.8, p97: 92.9 },
  { month: 24, p3: 81.7, p15: 84.7, p50: 87.8, p85: 90.9, p97: 93.9 },
];

// WHO Girls Length-for-age (cm), 0-24 months
export const whoHeightGirls: PercentileRow[] = [
  { month: 0, p3: 45.4, p15: 47.2, p50: 49.1, p85: 51.1, p97: 52.9 },
  { month: 1, p3: 49.8, p15: 51.8, p50: 53.7, p85: 55.7, p97: 57.6 },
  { month: 2, p3: 53.0, p15: 55.0, p50: 57.1, p85: 59.1, p97: 61.1 },
  { month: 3, p3: 55.6, p15: 57.7, p50: 59.8, p85: 61.9, p97: 63.9 },
  { month: 4, p3: 57.8, p15: 59.9, p50: 62.1, p85: 64.2, p97: 66.3 },
  { month: 5, p3: 59.6, p15: 61.8, p50: 64.0, p85: 66.2, p97: 68.4 },
  { month: 6, p3: 61.2, p15: 63.4, p50: 65.7, p85: 67.9, p97: 70.2 },
  { month: 7, p3: 62.7, p15: 65.0, p50: 67.3, p85: 69.6, p97: 71.9 },
  { month: 8, p3: 64.0, p15: 66.4, p50: 68.7, p85: 71.1, p97: 73.5 },
  { month: 9, p3: 65.3, p15: 67.7, p50: 70.1, p85: 72.6, p97: 74.9 },
  { month: 10, p3: 66.5, p15: 68.9, p50: 71.5, p85: 73.9, p97: 76.4 },
  { month: 11, p3: 67.7, p15: 70.2, p50: 72.8, p85: 75.3, p97: 77.8 },
  { month: 12, p3: 68.9, p15: 71.5, p50: 74.0, p85: 76.6, p97: 79.2 },
  { month: 13, p3: 70.0, p15: 72.6, p50: 75.2, p85: 77.8, p97: 80.5 },
  { month: 14, p3: 71.0, p15: 73.7, p50: 76.4, p85: 79.1, p97: 81.8 },
  { month: 15, p3: 72.0, p15: 74.8, p50: 77.5, p85: 80.3, p97: 83.1 },
  { month: 16, p3: 73.0, p15: 75.8, p50: 78.6, p85: 81.5, p97: 84.3 },
  { month: 17, p3: 73.9, p15: 76.8, p50: 79.7, p85: 82.6, p97: 85.5 },
  { month: 18, p3: 74.9, p15: 77.8, p50: 80.7, p85: 83.6, p97: 86.5 },
  { month: 19, p3: 75.7, p15: 78.7, p50: 81.7, p85: 84.7, p97: 87.7 },
  { month: 20, p3: 76.5, p15: 79.6, p50: 82.7, p85: 85.8, p97: 88.9 },
  { month: 21, p3: 77.3, p15: 80.5, p50: 83.6, p85: 86.7, p97: 90.0 },
  { month: 22, p3: 78.1, p15: 81.3, p50: 84.6, p85: 87.7, p97: 91.1 },
  { month: 23, p3: 78.9, p15: 82.1, p50: 85.5, p85: 88.7, p97: 92.2 },
  { month: 24, p3: 79.6, p15: 83.0, p50: 86.4, p85: 89.8, p97: 93.3 },
];

// WHO Boys Weight-for-age (kg), 0-24 months
export const whoWeightBoys: PercentileRow[] = [
  { month: 0, p3: 2.5, p15: 2.9, p50: 3.3, p85: 3.9, p97: 4.4 },
  { month: 1, p3: 3.4, p15: 3.9, p50: 4.5, p85: 5.1, p97: 5.8 },
  { month: 2, p3: 4.4, p15: 4.9, p50: 5.6, p85: 6.3, p97: 7.1 },
  { month: 3, p3: 5.1, p15: 5.7, p50: 6.4, p85: 7.2, p97: 8.0 },
  { month: 4, p3: 5.6, p15: 6.2, p50: 7.0, p85: 7.9, p97: 8.7 },
  { month: 5, p3: 6.1, p15: 6.7, p50: 7.5, p85: 8.4, p97: 9.3 },
  { month: 6, p3: 6.4, p15: 7.1, p50: 7.9, p85: 8.8, p97: 9.8 },
  { month: 7, p3: 6.7, p15: 7.4, p50: 8.3, p85: 9.2, p97: 10.3 },
  { month: 8, p3: 7.0, p15: 7.7, p50: 8.6, p85: 9.6, p97: 10.7 },
  { month: 9, p3: 7.2, p15: 8.0, p50: 8.9, p85: 9.9, p97: 11.0 },
  { month: 10, p3: 7.5, p15: 8.2, p50: 9.2, p85: 10.2, p97: 11.4 },
  { month: 11, p3: 7.7, p15: 8.4, p50: 9.4, p85: 10.5, p97: 11.7 },
  { month: 12, p3: 7.8, p15: 8.6, p50: 9.6, p85: 10.8, p97: 12.0 },
  { month: 13, p3: 8.0, p15: 8.8, p50: 9.9, p85: 11.1, p97: 12.3 },
  { month: 14, p3: 8.2, p15: 9.0, p50: 10.1, p85: 11.3, p97: 12.6 },
  { month: 15, p3: 8.4, p15: 9.2, p50: 10.3, p85: 11.6, p97: 12.9 },
  { month: 16, p3: 8.5, p15: 9.4, p50: 10.5, p85: 11.8, p97: 13.2 },
  { month: 17, p3: 8.7, p15: 9.6, p50: 10.8, p85: 12.1, p97: 13.5 },
  { month: 18, p3: 8.8, p15: 9.7, p50: 10.9, p85: 12.3, p97: 13.7 },
  { month: 19, p3: 9.0, p15: 9.9, p50: 11.1, p85: 12.5, p97: 14.0 },
  { month: 20, p3: 9.2, p15: 10.1, p50: 11.3, p85: 12.7, p97: 14.2 },
  { month: 21, p3: 9.3, p15: 10.3, p50: 11.5, p85: 13.0, p97: 14.5 },
  { month: 22, p3: 9.5, p15: 10.5, p50: 11.8, p85: 13.2, p97: 14.8 },
  { month: 23, p3: 9.7, p15: 10.7, p50: 12.0, p85: 13.5, p97: 15.1 },
  { month: 24, p3: 9.8, p15: 10.8, p50: 12.2, p85: 13.7, p97: 15.3 },
];

// WHO Girls Weight-for-age (kg), 0-24 months
export const whoWeightGirls: PercentileRow[] = [
  { month: 0, p3: 2.4, p15: 2.8, p50: 3.2, p85: 3.7, p97: 4.2 },
  { month: 1, p3: 3.2, p15: 3.6, p50: 4.2, p85: 4.8, p97: 5.5 },
  { month: 2, p3: 4.0, p15: 4.5, p50: 5.1, p85: 5.8, p97: 6.6 },
  { month: 3, p3: 4.6, p15: 5.2, p50: 5.8, p85: 6.6, p97: 7.5 },
  { month: 4, p3: 5.1, p15: 5.7, p50: 6.4, p85: 7.3, p97: 8.2 },
  { month: 5, p3: 5.5, p15: 6.1, p50: 6.9, p85: 7.8, p97: 8.8 },
  { month: 6, p3: 5.7, p15: 6.5, p50: 7.3, p85: 8.2, p97: 9.3 },
  { month: 7, p3: 6.0, p15: 6.8, p50: 7.6, p85: 8.6, p97: 9.8 },
  { month: 8, p3: 6.3, p15: 7.0, p50: 7.9, p85: 9.0, p97: 10.2 },
  { month: 9, p3: 6.5, p15: 7.3, p50: 8.2, p85: 9.3, p97: 10.5 },
  { month: 10, p3: 6.7, p15: 7.5, p50: 8.5, p85: 9.6, p97: 10.9 },
  { month: 11, p3: 6.9, p15: 7.7, p50: 8.7, p85: 9.9, p97: 11.2 },
  { month: 12, p3: 7.1, p15: 7.9, p50: 8.9, p85: 10.1, p97: 11.5 },
  { month: 13, p3: 7.2, p15: 8.1, p50: 9.2, p85: 10.4, p97: 11.8 },
  { month: 14, p3: 7.4, p15: 8.3, p50: 9.4, p85: 10.7, p97: 12.1 },
  { month: 15, p3: 7.6, p15: 8.5, p50: 9.6, p85: 10.9, p97: 12.4 },
  { month: 16, p3: 7.7, p15: 8.7, p50: 9.8, p85: 11.2, p97: 12.7 },
  { month: 17, p3: 7.9, p15: 8.9, p50: 10.0, p85: 11.4, p97: 13.0 },
  { month: 18, p3: 8.1, p15: 9.1, p50: 10.2, p85: 11.7, p97: 13.2 },
  { month: 19, p3: 8.2, p15: 9.2, p50: 10.4, p85: 11.9, p97: 13.5 },
  { month: 20, p3: 8.4, p15: 9.4, p50: 10.6, p85: 12.1, p97: 13.7 },
  { month: 21, p3: 8.6, p15: 9.6, p50: 10.9, p85: 12.4, p97: 14.0 },
  { month: 22, p3: 8.7, p15: 9.8, p50: 11.1, p85: 12.6, p97: 14.3 },
  { month: 23, p3: 8.9, p15: 10.0, p50: 11.3, p85: 12.9, p97: 14.6 },
  { month: 24, p3: 9.0, p15: 10.2, p50: 11.5, p85: 13.1, p97: 14.9 },
];

// CDC NCHS 2000 Boys Stature-for-age (cm), annual from 24-240 months (2-20 yr)
export const cdcHeightBoys: PercentileRow[] = [
  { month: 24, p3: 82.3, p15: 85.1, p50: 87.6, p85: 90.2, p97: 93.0 },
  { month: 36, p3: 89.0, p15: 92.4, p50: 95.7, p85: 98.9, p97: 101.9 },
  { month: 48, p3: 95.4, p15: 99.1, p50: 102.9, p85: 106.6, p97: 110.1 },
  { month: 60, p3: 101.5, p15: 105.5, p50: 109.4, p85: 113.5, p97: 117.4 },
  { month: 72, p3: 107.1, p15: 111.4, p50: 115.5, p85: 119.8, p97: 124.1 },
  { month: 84, p3: 112.1, p15: 116.7, p50: 121.1, p85: 125.7, p97: 130.4 },
  { month: 96, p3: 117.0, p15: 121.8, p50: 126.6, p85: 131.6, p97: 136.8 },
  { month: 108, p3: 121.5, p15: 126.6, p50: 131.8, p85: 137.3, p97: 142.9 },
  { month: 120, p3: 125.7, p15: 131.1, p50: 136.9, p85: 142.9, p97: 149.2 },
  { month: 132, p3: 129.5, p15: 135.3, p50: 141.6, p85: 148.3, p97: 155.3 },
  { month: 144, p3: 133.5, p15: 139.7, p50: 146.8, p85: 154.4, p97: 162.1 },
  { month: 156, p3: 138.8, p15: 145.8, p50: 153.5, p85: 161.4, p97: 169.0 },
  { month: 168, p3: 144.6, p15: 151.9, p50: 159.9, p85: 167.9, p97: 175.3 },
  { month: 180, p3: 150.0, p15: 157.4, p50: 165.3, p85: 172.9, p97: 179.8 },
  { month: 192, p3: 154.0, p15: 161.3, p50: 169.2, p85: 176.6, p97: 183.0 },
  { month: 204, p3: 156.6, p15: 163.7, p50: 171.6, p85: 178.7, p97: 184.9 },
  { month: 216, p3: 158.1, p15: 165.1, p50: 173.1, p85: 179.9, p97: 185.8 },
  { month: 228, p3: 158.9, p15: 165.9, p50: 173.9, p85: 180.5, p97: 186.2 },
  { month: 240, p3: 159.0, p15: 166.0, p50: 174.0, p85: 180.6, p97: 186.2 },
];

// CDC NCHS 2000 Girls Stature-for-age (cm), annual from 24-240 months (2-20 yr)
export const cdcHeightGirls: PercentileRow[] = [
  { month: 24, p3: 80.9, p15: 83.7, p50: 86.4, p85: 89.1, p97: 91.9 },
  { month: 36, p3: 88.3, p15: 91.7, p50: 94.9, p85: 98.2, p97: 101.5 },
  { month: 48, p3: 94.4, p15: 98.0, p50: 101.6, p85: 105.3, p97: 109.0 },
  { month: 60, p3: 100.1, p15: 104.0, p50: 107.9, p85: 111.9, p97: 115.9 },
  { month: 72, p3: 105.4, p15: 109.6, p50: 113.7, p85: 117.9, p97: 122.2 },
  { month: 84, p3: 110.5, p15: 115.0, p50: 119.4, p85: 124.0, p97: 128.9 },
  { month: 96, p3: 115.4, p15: 120.2, p50: 124.9, p85: 129.9, p97: 135.2 },
  { month: 108, p3: 120.1, p15: 125.2, p50: 130.3, p85: 135.7, p97: 141.4 },
  { month: 120, p3: 124.6, p15: 130.2, p50: 135.7, p85: 141.7, p97: 148.1 },
  { month: 132, p3: 129.3, p15: 135.4, p50: 141.7, p85: 148.5, p97: 155.7 },
  { month: 144, p3: 134.9, p15: 141.5, p50: 148.3, p85: 155.4, p97: 162.7 },
  { month: 156, p3: 140.0, p15: 146.8, p50: 153.7, p85: 160.5, p97: 167.3 },
  { month: 168, p3: 143.9, p15: 150.5, p50: 157.2, p85: 163.6, p97: 170.2 },
  { month: 180, p3: 146.4, p15: 152.7, p50: 159.2, p85: 165.5, p97: 171.8 },
  { month: 192, p3: 147.7, p15: 153.8, p50: 160.2, p85: 166.4, p97: 172.5 },
  { month: 204, p3: 148.3, p15: 154.4, p50: 160.8, p85: 166.9, p97: 173.0 },
  { month: 216, p3: 148.5, p15: 154.7, p50: 161.2, p85: 167.3, p97: 173.4 },
  { month: 228, p3: 148.7, p15: 154.9, p50: 161.3, p85: 167.5, p97: 173.5 },
  { month: 240, p3: 148.7, p15: 154.9, p50: 161.3, p85: 167.5, p97: 173.5 },
];

// CDC NCHS 2000 Boys Weight-for-age (kg), annual from 24-240 months (2-20 yr)
export const cdcWeightBoys: PercentileRow[] = [
  { month: 24, p3: 10.5, p15: 11.6, p50: 12.9, p85: 14.5, p97: 16.4 },
  { month: 36, p3: 12.1, p15: 13.5, p50: 14.9, p85: 17.1, p97: 19.7 },
  { month: 48, p3: 13.7, p15: 15.3, p50: 16.9, p85: 19.7, p97: 22.7 },
  { month: 60, p3: 15.3, p15: 17.1, p50: 19.0, p85: 22.2, p97: 25.8 },
  { month: 72, p3: 17.0, p15: 19.1, p50: 21.5, p85: 25.3, p97: 29.7 },
  { month: 84, p3: 18.8, p15: 21.2, p50: 24.2, p85: 28.9, p97: 34.7 },
  { month: 96, p3: 20.6, p15: 23.5, p50: 27.3, p85: 32.8, p97: 40.3 },
  { month: 108, p3: 22.4, p15: 25.9, p50: 30.7, p85: 37.5, p97: 46.7 },
  { month: 120, p3: 24.4, p15: 28.5, p50: 34.3, p85: 42.4, p97: 53.8 },
  { month: 132, p3: 26.7, p15: 31.5, p50: 38.3, p85: 47.7, p97: 61.0 },
  { month: 144, p3: 29.7, p15: 35.4, p50: 43.1, p85: 53.6, p97: 68.5 },
  { month: 156, p3: 33.8, p15: 40.3, p50: 48.9, p85: 60.3, p97: 76.4 },
  { month: 168, p3: 38.5, p15: 45.6, p50: 55.0, p85: 67.1, p97: 84.3 },
  { month: 180, p3: 43.1, p15: 50.7, p50: 60.7, p85: 73.1, p97: 91.1 },
  { month: 192, p3: 47.2, p15: 55.2, p50: 65.5, p85: 78.3, p97: 97.1 },
  { month: 204, p3: 50.8, p15: 59.1, p50: 69.8, p85: 83.2, p97: 102.8 },
  { month: 216, p3: 53.8, p15: 62.5, p50: 73.5, p85: 87.5, p97: 107.9 },
  { month: 228, p3: 56.0, p15: 65.0, p50: 76.3, p85: 90.8, p97: 111.9 },
  { month: 240, p3: 57.4, p15: 66.7, p50: 78.1, p85: 92.8, p97: 114.8 },
];

// CDC NCHS 2000 Girls Weight-for-age (kg), annual from 24-240 months (2-20 yr)
export const cdcWeightGirls: PercentileRow[] = [
  { month: 24, p3: 10.1, p15: 11.2, p50: 12.5, p85: 14.0, p97: 15.9 },
  { month: 36, p3: 11.6, p15: 13.0, p50: 14.6, p85: 16.5, p97: 18.9 },
  { month: 48, p3: 13.1, p15: 14.8, p50: 16.7, p85: 19.1, p97: 22.0 },
  { month: 60, p3: 14.6, p15: 16.5, p50: 18.8, p85: 21.8, p97: 25.5 },
  { month: 72, p3: 16.1, p15: 18.2, p50: 21.0, p85: 24.5, p97: 29.1 },
  { month: 84, p3: 17.6, p15: 20.0, p50: 23.5, p85: 27.7, p97: 33.4 },
  { month: 96, p3: 19.4, p15: 22.2, p50: 26.5, p85: 31.6, p97: 38.6 },
  { month: 108, p3: 21.2, p15: 24.7, p50: 29.7, p85: 36.1, p97: 44.8 },
  { month: 120, p3: 23.1, p15: 27.3, p50: 33.1, p85: 40.7, p97: 51.1 },
  { month: 132, p3: 25.3, p15: 30.2, p50: 36.9, p85: 45.6, p97: 57.8 },
  { month: 144, p3: 27.8, p15: 33.4, p50: 41.0, p85: 50.8, p97: 64.6 },
  { month: 156, p3: 30.5, p15: 36.7, p50: 44.9, p85: 55.8, p97: 71.2 },
  { month: 168, p3: 33.2, p15: 39.8, p50: 48.5, p85: 60.4, p97: 77.2 },
  { month: 180, p3: 35.4, p15: 42.4, p50: 51.7, p85: 64.5, p97: 82.8 },
  { month: 192, p3: 37.3, p15: 44.5, p50: 54.3, p85: 68.0, p97: 87.6 },
  { month: 204, p3: 38.7, p15: 46.2, p50: 56.5, p85: 70.7, p97: 91.5 },
  { month: 216, p3: 39.8, p15: 47.7, p50: 58.2, p85: 73.0, p97: 95.1 },
  { month: 228, p3: 40.7, p15: 48.9, p50: 59.7, p85: 75.0, p97: 98.1 },
  { month: 240, p3: 41.5, p15: 50.1, p50: 61.2, p85: 76.9, p97: 101.1 },
];

/** Linear interpolation between two adjacent rows in a percentile table. */
export function interpolatePercentiles(table: PercentileRow[], ageMonths: number): PercentileRow | null {
  if (table.length === 0) return null;
  if (ageMonths <= table[0].month) return table[0];
  if (ageMonths >= table[table.length - 1].month) return table[table.length - 1];

  for (let i = 0; i < table.length - 1; i++) {
    const lo = table[i];
    const hi = table[i + 1];
    if (ageMonths >= lo.month && ageMonths <= hi.month) {
      const t = (ageMonths - lo.month) / (hi.month - lo.month);
      return {
        month: ageMonths,
        p3: lo.p3 + t * (hi.p3 - lo.p3),
        p15: lo.p15 + t * (hi.p15 - lo.p15),
        p50: lo.p50 + t * (hi.p50 - lo.p50),
        p85: lo.p85 + t * (hi.p85 - lo.p85),
        p97: lo.p97 + t * (hi.p97 - lo.p97),
      };
    }
  }
  return null;
}

/** Pick the right WHO/CDC table and interpolate. Returns null if age is out of range (>240 months). */
export function getPercentileRow(
  ageMonths: number,
  gender: number, // 0=Male, 1=Female, 2=Unknown
  type: "height" | "weight"
): PercentileRow | null {
  if (ageMonths < 0 || ageMonths > 240) return null;

  const useWHO = ageMonths <= 24;

  let maleTable: PercentileRow[];
  let femaleTable: PercentileRow[];

  if (type === "height") {
    maleTable = useWHO ? whoHeightBoys : cdcHeightBoys;
    femaleTable = useWHO ? whoHeightGirls : cdcHeightGirls;
  } else {
    maleTable = useWHO ? whoWeightBoys : cdcWeightBoys;
    femaleTable = useWHO ? whoWeightGirls : cdcWeightGirls;
  }

  if (gender === 0) return interpolatePercentiles(maleTable, ageMonths);
  if (gender === 1) return interpolatePercentiles(femaleTable, ageMonths);

  // Unknown: average male and female
  const m = interpolatePercentiles(maleTable, ageMonths);
  const f = interpolatePercentiles(femaleTable, ageMonths);
  if (!m || !f) return m || f;
  return {
    month: ageMonths,
    p3: (m.p3 + f.p3) / 2,
    p15: (m.p15 + f.p15) / 2,
    p50: (m.p50 + f.p50) / 2,
    p85: (m.p85 + f.p85) / 2,
    p97: (m.p97 + f.p97) / 2,
  };
}

/** Given a value and a percentile row, returns the approximate percentile rank (3-97). */
function approxPercentileRank(value: number, row: PercentileRow): number | "below" | "above" {
  if (value < row.p3) return "below";
  if (value > row.p97) return "above";

  const segments = [
    { lo: row.p3, hi: row.p15, pLo: 3, pHi: 15 },
    { lo: row.p15, hi: row.p50, pLo: 15, pHi: 50 },
    { lo: row.p50, hi: row.p85, pLo: 50, pHi: 85 },
    { lo: row.p85, hi: row.p97, pLo: 85, pHi: 97 },
  ];

  for (const seg of segments) {
    if (value >= seg.lo && value <= seg.hi) {
      const span = seg.hi - seg.lo;
      if (span < 1e-9) return seg.pLo;
      const t = (value - seg.lo) / span;
      return seg.pLo + t * (seg.pHi - seg.pLo);
    }
  }
  return 50;
}

function ordinalSuffix(n: number): string {
  if (n === 1) return "st";
  if (n === 2) return "nd";
  if (n === 3) return "rd";
  return "th";
}

/**
 * Returns a human-readable percentile label like "~75th %ile", "<3rd %ile", or ">97th %ile".
 * Returns null if the age is out of range (>240 months / >20 years) or birthday is not provided.
 *
 * @param value   Raw measurement value
 * @param unit    Unit of value ("cm", "in", "kg", "lbs")
 * @param ageMonths  Age in months at measurement time
 * @param gender  0=Male, 1=Female, 2=Unknown
 * @param type    "height" or "weight"
 */
export function computePercentileLabel(
  value: number,
  unit: string,
  ageMonths: number,
  gender: number,
  type: "height" | "weight"
): string | null {
  if (ageMonths < 0 || ageMonths > 240) return null;

  // Normalize to metric
  let normalized = value;
  if (unit === "in") normalized = value * 2.54;
  else if (unit === "lbs") normalized = value * 0.453592;

  const row = getPercentileRow(ageMonths, gender, type);
  if (!row) return null;

  const rank = approxPercentileRank(normalized, row);
  if (rank === "below") return "<3rd %ile";
  if (rank === "above") return ">97th %ile";

  const rounded = Math.round(rank);
  return `~${rounded}${ordinalSuffix(rounded)} %ile`;
}

/** Returns true if birthday is a real date (not Go's zero time "0001-01-01"). */
export function isValidBirthday(birthday: string | undefined | null): birthday is string {
  if (!birthday) return false;
  const year = new Date(birthday).getFullYear();
  return year > 1000;
}

/** Calculate age in months between two dates. */
export function ageInMonths(birthday: string | Date, measurementDate: string | Date): number {
  const birth = new Date(birthday);
  const measure = new Date(measurementDate);
  const yearDiff = measure.getFullYear() - birth.getFullYear();
  const monthDiff = measure.getMonth() - birth.getMonth();
  const dayDiff = measure.getDate() - birth.getDate();
  return yearDiff * 12 + monthDiff + dayDiff / 30.4375;
}

/** Format an age in months as a human-readable string. */
export function formatAgeAtMeasurement(ageMonths: number): string {
  if (ageMonths < 0) return "—";
  if (ageMonths < 24) {
    const mo = Math.round(ageMonths);
    return mo === 1 ? "1 mo" : `${mo} mo`;
  }
  const totalMonths = Math.round(ageMonths);
  const yrs = Math.floor(totalMonths / 12);
  const mos = totalMonths % 12;
  if (mos === 0) return yrs === 1 ? "1 yr" : `${yrs} yr`;
  return `${yrs} yr ${mos} mo`;
}
