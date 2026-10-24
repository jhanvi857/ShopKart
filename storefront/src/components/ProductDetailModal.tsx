import React, { useEffect, useState } from 'react';
import { Product, Review } from '../types';
import { api } from '../api';
import { X, Star, ShoppingCart, Check, ShieldCheck, Truck, RefreshCw } from 'lucide-react';

interface ProductDetailModalProps {
  product: Product | null;
  stock?: number;
  onClose: () => void;
  onAddToCart: (p: Product) => void;
}

export const ProductDetailModal: React.FC<ProductDetailModalProps> = ({
  product,
  stock,
  onClose,
  onAddToCart,
}) => {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [isLoadingReviews, setIsLoadingReviews] = useState(false);
  const [author, setAuthor] = useState('');
  const [rating, setRating] = useState(5);
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [added, setAdded] = useState(false);

  useEffect(() => {
    if (product) {
      setIsLoadingReviews(true);
      api.getReviews(product.id)
        .then((data) => setReviews(data))
        .catch(() => setReviews([]))
        .finally(() => setIsLoadingReviews(false));
    }
  }, [product]);

  if (!product) return null;

  const isOutOfStock = stock !== undefined && stock <= 0;
  const isLowStock = stock !== undefined && stock > 0 && stock <= 5;
  const discountPercent = product.mrp > product.price
    ? Math.round(((product.mrp - product.price) / product.mrp) * 100)
    : 0;

  const handleAdd = () => {
    onAddToCart(product);
    setAdded(true);
    setTimeout(() => setAdded(false), 1500);
  };

  const handleReviewSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!comment || !author) return;

    try {
      setIsSubmitting(true);
      const newRev = await api.submitReview({
        product_id: product.id,
        rating,
        comment,
        author,
      });
      setReviews([newRev, ...reviews]);
      setComment('');
      setAuthor('');
    } catch (err: any) {
      alert(`Could not submit review: ${err.message}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 overflow-y-auto">
      <div className="relative w-full max-w-3xl bg-white rounded border border-[#e0e0e0] shadow-xl overflow-hidden my-8">
        {/* Close Button */}
        <button
          onClick={onClose}
          className="absolute top-3 right-3 z-10 p-1.5 text-slate-500 hover:text-[#212121] bg-slate-100 hover:bg-slate-200 rounded transition"
          aria-label="Close"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="grid grid-cols-1 md:grid-cols-2 p-6 gap-6">
          {/* Product Media */}
          <div className="flex flex-col items-center">
            <div className="h-72 w-full bg-[#f8f9fa] border border-[#e0e0e0] rounded flex items-center justify-center p-4">
              <img
                src={product.image_url}
                alt={product.name}
                className="max-h-full max-w-full object-contain"
                onError={(e) => {
                  (e.target as HTMLImageElement).src = 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=500&auto=format&fit=crop&q=75';
                }}
              />
            </div>

            {/* Quick Guarantees */}
            <div className="grid grid-cols-3 gap-2 mt-4 w-full text-center text-[10px] text-slate-600">
              <div className="p-2 bg-slate-50 border border-slate-100 rounded flex flex-col items-center">
                <Truck className="w-4 h-4 text-[#2874f0] mb-1" />
                <span>Free Delivery</span>
              </div>
              <div className="p-2 bg-slate-50 border border-slate-100 rounded flex flex-col items-center">
                <ShieldCheck className="w-4 h-4 text-[#388e3c] mb-1" />
                <span>1 Year Warranty</span>
              </div>
              <div className="p-2 bg-slate-50 border border-slate-100 rounded flex flex-col items-center">
                <RefreshCw className="w-4 h-4 text-amber-600 mb-1" />
                <span>7-Day Replacement</span>
              </div>
            </div>
          </div>

          {/* Details Column */}
          <div className="flex flex-col justify-between">
            <div>
              <span className="text-[11px] font-bold text-slate-500 uppercase tracking-widest">
                {product.brand}
              </span>
              <h2 className="text-lg font-bold text-[#212121] leading-snug mt-1">
                {product.name}
              </h2>

              {/* Rating pill */}
              <div className="flex items-center gap-2 mt-2">
                <div className="flex items-center gap-1 bg-[#388e3c] text-white text-xs font-bold px-2 py-0.5 rounded-sm">
                  <span>{product.rating.toFixed(1)}</span>
                  <Star className="w-3 h-3 fill-current" />
                </div>
                <span className="text-xs text-slate-500 font-medium">
                  {product.rating_count.toLocaleString('en-IN')} Ratings & {reviews.length} Reviews
                </span>
              </div>

              {/* Price & Savings */}
              <div className="mt-4 pt-3 border-t border-[#e0e0e0]">
                <div className="flex items-baseline gap-2">
                  <span className="text-2xl font-bold text-[#212121]">
                    ₹{product.price.toLocaleString('en-IN')}
                  </span>
                  {product.mrp > product.price && (
                    <>
                      <span className="text-sm text-slate-400 line-through">
                        ₹{product.mrp.toLocaleString('en-IN')}
                      </span>
                      <span className="text-sm font-bold text-[#388e3c]">
                        {discountPercent}% off
                      </span>
                    </>
                  )}
                </div>
                <span className="text-[11px] text-slate-500">Inclusive of all taxes</span>
              </div>

              {/* Stock Status */}
              <div className="mt-3 text-xs">
                {isOutOfStock ? (
                  <span className="text-rose-600 font-bold">Currently Out of Stock</span>
                ) : isLowStock ? (
                  <span className="text-amber-700 font-bold">Hurry! Only {stock} units left in stock</span>
                ) : (
                  <span className="text-[#388e3c] font-semibold">In Stock — Dispatched in 24 hours</span>
                )}
              </div>

              {/* Product Specifications Table */}
              <div className="mt-4">
                <h4 className="text-xs font-bold text-slate-700 uppercase tracking-wider mb-1.5">
                  Specifications
                </h4>
                <div className="bg-slate-50 border border-[#e0e0e0] rounded p-2.5 text-xs text-slate-700 space-y-1">
                  <div className="flex justify-between py-0.5 border-b border-slate-200">
                    <span className="text-slate-500">Category:</span>
                    <span className="font-medium text-[#212121]">{product.category}</span>
                  </div>
                  <div className="flex justify-between py-0.5 border-b border-slate-200">
                    <span className="text-slate-500">Technical Specs:</span>
                    <span className="font-medium text-[#212121] text-right max-w-xs">{product.specs || 'Standard Retail Grade'}</span>
                  </div>
                  <div className="flex justify-between py-0.5">
                    <span className="text-slate-500">Overview:</span>
                    <span className="font-medium text-[#212121] text-right max-w-xs">{product.description}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Action Button */}
            <div className="mt-6 pt-4 border-t border-[#e0e0e0]">
              <button
                onClick={handleAdd}
                disabled={isOutOfStock}
                className={`w-full py-2.5 px-4 rounded text-sm font-bold flex items-center justify-center gap-2 transition ${
                  isOutOfStock
                    ? 'bg-slate-100 text-slate-400 cursor-not-allowed border border-slate-200'
                    : added
                    ? 'bg-[#388e3c] text-white'
                    : 'bg-[#ff9f00] hover:bg-[#f39700] text-white'
                }`}
              >
                {added ? (
                  <>
                    <Check className="w-4 h-4 text-white" />
                    <span>Added to Cart!</span>
                  </>
                ) : (
                  <>
                    <ShoppingCart className="w-4 h-4" />
                    <span>{isOutOfStock ? 'Out of Stock' : `Add to Cart - ₹${product.price.toLocaleString('en-IN')}`}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>

        {/* Customer Reviews Section */}
        <div className="bg-[#f8f9fa] border-t border-[#e0e0e0] p-6 text-xs">
          <h4 className="font-bold text-[#212121] uppercase tracking-wider mb-3">
            Ratings & Reviews ({reviews.length})
          </h4>

          {/* Reviews List */}
          <div className="max-h-40 overflow-y-auto space-y-2 pr-1 mb-4">
            {isLoadingReviews ? (
              <p className="text-slate-400">Loading verified customer reviews...</p>
            ) : reviews.length === 0 ? (
              <p className="text-slate-400">No verified reviews yet. Be the first to review this product!</p>
            ) : (
              reviews.map((r) => (
                <div key={r.id} className="p-2.5 bg-white border border-[#e0e0e0] rounded">
                  <div className="flex items-center justify-between mb-1">
                    <span className="font-semibold text-[#212121]">{r.author}</span>
                    <div className="flex items-center gap-0.5 bg-[#388e3c] text-white text-[10px] font-bold px-1.5 py-0.5 rounded-sm">
                      <span>{r.rating}</span>
                      <Star className="w-2.5 h-2.5 fill-current" />
                    </div>
                  </div>
                  <p className="text-slate-600">{r.comment}</p>
                </div>
              ))
            )}
          </div>

          {/* Add Review Form */}
          <form onSubmit={handleReviewSubmit} className="bg-white border border-[#e0e0e0] rounded p-3 grid grid-cols-1 sm:grid-cols-3 gap-2">
            <input
              type="text"
              placeholder="Your Name (e.g. Ramesh Kumar)"
              value={author}
              onChange={(e) => setAuthor(e.target.value)}
              required
              className="px-3 py-1.5 border border-[#e0e0e0] rounded text-xs focus:outline-none focus:border-[#2874f0]"
            />
            <input
              type="text"
              placeholder="Write your honest review..."
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              required
              className="px-3 py-1.5 border border-[#e0e0e0] rounded text-xs focus:outline-none focus:border-[#2874f0] sm:col-span-2"
            />
            <div className="flex items-center justify-between sm:col-span-3 mt-1">
              <div className="flex items-center gap-1">
                <span className="text-slate-600 font-medium">Rating:</span>
                {[1, 2, 3, 4, 5].map((star) => (
                  <button
                    type="button"
                    key={star}
                    onClick={() => setRating(star)}
                    className="p-0.5 text-amber-500"
                  >
                    <Star className={`w-3.5 h-3.5 ${star <= rating ? 'fill-current' : 'text-slate-300'}`} />
                  </button>
                ))}
              </div>
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-3 py-1 bg-[#2874f0] hover:bg-[#1f5ec7] text-white font-semibold rounded text-xs disabled:opacity-50"
              >
                {isSubmitting ? 'Submitting...' : 'Submit Review'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};
