import React from 'react';
import { Star, RotateCcw } from 'lucide-react';

interface FilterSidebarProps {
  categories: string[];
  selectedCategory: string;
  onSelectCategory: (cat: string) => void;
  priceRange: string;
  onSelectPriceRange: (range: string) => void;
  minRating: number;
  onSelectMinRating: (rating: number) => void;
  brands: string[];
  selectedBrand: string;
  onSelectBrand: (brand: string) => void;
  inStockOnly: boolean;
  onToggleInStock: (val: boolean) => void;
  onResetFilters: () => void;
  totalResults: number;
}

export const FilterSidebar: React.FC<FilterSidebarProps> = ({
  categories,
  selectedCategory,
  onSelectCategory,
  priceRange,
  onSelectPriceRange,
  minRating,
  onSelectMinRating,
  brands,
  selectedBrand,
  onSelectBrand,
  inStockOnly,
  onToggleInStock,
  onResetFilters,
  totalResults,
}) => {
  return (
    <aside className="w-full lg:w-60 bg-white border border-[#e0e0e0] rounded p-4 text-xs select-none">
      <div className="flex items-center justify-between border-b border-[#e0e0e0] pb-3 mb-4">
        <h3 className="font-bold text-sm text-[#212121] uppercase tracking-wide">Filters</h3>
        <button
          onClick={onResetFilters}
          className="text-[#2874f0] font-semibold text-xs flex items-center gap-1 hover:underline"
        >
          <RotateCcw className="w-3 h-3" />
          <span>Clear All</span>
        </button>
      </div>

      {/* Category Section */}
      <div className="border-b border-[#e0e0e0] pb-3 mb-3">
        <h4 className="font-bold text-[#212121] mb-2 uppercase text-[11px] tracking-wider text-slate-500">
          Categories
        </h4>
        <div className="space-y-1.5 max-h-40 overflow-y-auto pr-1">
          <label className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
            <input
              type="radio"
              name="filter-cat"
              checked={selectedCategory === ''}
              onChange={() => onSelectCategory('')}
              className="accent-[#2874f0]"
            />
            <span className={selectedCategory === '' ? 'font-bold text-[#2874f0]' : ''}>
              All Categories
            </span>
          </label>
          {categories.map((cat) => (
            <label key={cat} className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
              <input
                type="radio"
                name="filter-cat"
                checked={selectedCategory === cat}
                onChange={() => onSelectCategory(cat)}
                className="accent-[#2874f0]"
              />
              <span className={selectedCategory === cat ? 'font-bold text-[#2874f0]' : ''}>
                {cat}
              </span>
            </label>
          ))}
        </div>
      </div>

      {/* Price Range */}
      <div className="border-b border-[#e0e0e0] pb-3 mb-3">
        <h4 className="font-bold text-[#212121] mb-2 uppercase text-[11px] tracking-wider text-slate-500">
          Price Range (INR)
        </h4>
        <div className="space-y-1.5">
          {[
            { id: 'all', label: 'All Prices' },
            { id: 'under-500', label: 'Under ₹500' },
            { id: '500-1000', label: '₹500 - ₹1,000' },
            { id: '1000-2500', label: '₹1,000 - ₹2,500' },
            { id: 'above-2500', label: '₹2,500 & Above' },
          ].map((opt) => (
            <label key={opt.id} className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
              <input
                type="radio"
                name="filter-price"
                checked={priceRange === opt.id}
                onChange={() => onSelectPriceRange(opt.id)}
                className="accent-[#2874f0]"
              />
              <span className={priceRange === opt.id ? 'font-bold text-[#2874f0]' : ''}>
                {opt.label}
              </span>
            </label>
          ))}
        </div>
      </div>

      {/* Customer Ratings */}
      <div className="border-b border-[#e0e0e0] pb-3 mb-3">
        <h4 className="font-bold text-[#212121] mb-2 uppercase text-[11px] tracking-wider text-slate-500">
          Customer Rating
        </h4>
        <div className="space-y-1.5">
          {[
            { val: 0, label: 'All Ratings' },
            { val: 4, label: '4★ & above' },
            { val: 3, label: '3★ & above' },
          ].map((opt) => (
            <label key={opt.val} className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
              <input
                type="radio"
                name="filter-rating"
                checked={minRating === opt.val}
                onChange={() => onSelectMinRating(opt.val)}
                className="accent-[#2874f0]"
              />
              <span className="flex items-center gap-1">
                {opt.label}
              </span>
            </label>
          ))}
        </div>
      </div>

      {/* Brands */}
      {brands.length > 0 && (
        <div className="border-b border-[#e0e0e0] pb-3 mb-3">
          <h4 className="font-bold text-[#212121] mb-2 uppercase text-[11px] tracking-wider text-slate-500">
            Brand
          </h4>
          <div className="space-y-1.5 max-h-36 overflow-y-auto pr-1">
            <label className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
              <input
                type="radio"
                name="filter-brand"
                checked={selectedBrand === ''}
                onChange={() => onSelectBrand('')}
                className="accent-[#2874f0]"
              />
              <span className={selectedBrand === '' ? 'font-bold text-[#2874f0]' : ''}>All Brands</span>
            </label>
            {brands.map((b) => (
              <label key={b} className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
                <input
                  type="radio"
                  name="filter-brand"
                  checked={selectedBrand === b}
                  onChange={() => onSelectBrand(b)}
                  className="accent-[#2874f0]"
                />
                <span className={selectedBrand === b ? 'font-bold text-[#2874f0]' : ''}>{b}</span>
              </label>
            ))}
          </div>
        </div>
      )}

      {/* Availability */}
      <div>
        <h4 className="font-bold text-[#212121] mb-2 uppercase text-[11px] tracking-wider text-slate-500">
          Availability
        </h4>
        <label className="flex items-center gap-2 cursor-pointer hover:text-[#2874f0]">
          <input
            type="checkbox"
            checked={inStockOnly}
            onChange={(e) => onToggleInStock(e.target.checked)}
            className="accent-[#2874f0]"
          />
          <span>Exclude Out of Stock</span>
        </label>
      </div>
    </aside>
  );
};
