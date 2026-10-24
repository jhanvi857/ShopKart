import React, { useEffect, useState } from 'react';
import { Product, Review } from '../types';
import { api } from '../api';
import { X, Star, ShoppingCart, MessageSquare, Check, ShieldCheck } from 'lucide-react';

interface ProductModalProps {
  product: Product | null;
  onClose: () => void;
  onAddToCart: (p: Product) => void;
}

export const ProductModal: React.FC<ProductModalProps> = ({
  product,
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-xs overflow-y-auto">
      <div className="relative w-full max-w-2xl bg-white rounded-3xl shadow-2xl border border-slate-200 overflow-hidden my-8">
        {/* Close Button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 z-10 p-2 bg-white/80 hover:bg-white rounded-full text-slate-500 hover:text-slate-900 shadow-xs transition"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="grid grid-cols-1 md:grid-cols-2">
          {/* Product Image */}
          <div className="h-64 md:h-full bg-slate-100 relative">
            <img
              src={product.image_url}
              alt={product.name}
              className="w-full h-full object-cover"
              onError={(e) => {
                (e.target as HTMLImageElement).src = 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=600&auto=format&fit=crop&q=80';
              }}
            />
            <span className="absolute bottom-4 left-4 px-3 py-1 bg-white/90 backdrop-blur-xs rounded-full text-xs font-semibold uppercase tracking-wider text-slate-800">
              {product.category}
            </span>
          </div>

          {/* Details */}
          <div className="p-6 flex flex-col justify-between">
            <div>
              <span className="text-[11px] font-bold text-emerald-600 uppercase tracking-widest">
                Product ID: {product.id}
              </span>
              <h2 className="text-xl font-bold text-slate-900 mt-1">{product.name}</h2>
              <div className="mt-2 text-2xl font-extrabold text-slate-900">
                ${product.price.toFixed(2)}
              </div>

              <p className="mt-4 text-sm text-slate-600 leading-relaxed">
                {product.description}
              </p>

              <div className="mt-6 flex items-center gap-2 text-xs text-slate-500">
                <ShieldCheck className="w-4 h-4 text-emerald-600" />
                <span>Verified Stock in Central Fulfillment Center</span>
              </div>
            </div>

            <div className="mt-8">
              <button
                onClick={handleAdd}
                className="w-full flex items-center justify-center gap-2 py-3 px-4 bg-emerald-600 hover:bg-emerald-700 text-white font-semibold rounded-xl shadow-md transition"
              >
                {added ? (
                  <>
                    <Check className="w-5 h-5 text-emerald-200" />
                    <span>Added to Cart!</span>
                  </>
                ) : (
                  <>
                    <ShoppingCart className="w-5 h-5" />
                    <span>Add to Cart - ${product.price.toFixed(2)}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>

        {/* Reviews Section */}
        <div className="p-6 bg-slate-50 border-t border-slate-100">
          <div className="flex items-center gap-2 mb-4">
            <MessageSquare className="w-4 h-4 text-emerald-600" />
            <h4 className="font-semibold text-slate-900 text-sm">Customer Reviews ({reviews.length})</h4>
          </div>

          {/* Reviews list */}
          <div className="max-h-40 overflow-y-auto space-y-3 pr-2 mb-6 text-xs">
            {isLoadingReviews ? (
              <p className="text-slate-400">Loading reviews...</p>
            ) : reviews.length === 0 ? (
              <p className="text-slate-400">No customer reviews yet. Be the first to review!</p>
            ) : (
              reviews.map((r) => (
                <div key={r.id} className="p-3 bg-white rounded-xl border border-slate-200/60 shadow-2xs">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-slate-800">{r.author}</span>
                    <div className="flex text-amber-400">
                      {[...Array(5)].map((_, i) => (
                        <Star key={i} className={`w-3 h-3 ${i < r.rating ? 'fill-current' : 'text-slate-200'}`} />
                      ))}
                    </div>
                  </div>
                  <p className="mt-1 text-slate-600">{r.comment}</p>
                </div>
              ))
            )}
          </div>

          {/* Add review form */}
          <form onSubmit={handleReviewSubmit} className="grid grid-cols-1 sm:grid-cols-3 gap-2">
            <input
              type="text"
              placeholder="Your name"
              value={author}
              onChange={(e) => setAuthor(e.target.value)}
              required
              className="px-3 py-2 text-xs bg-white border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-emerald-500"
            />
            <input
              type="text"
              placeholder="Your review"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              required
              className="px-3 py-2 text-xs bg-white border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:col-span-2"
            />
            <div className="flex items-center justify-between sm:col-span-3 mt-1">
              <div className="flex items-center gap-1 text-xs text-slate-600">
                <span>Rating:</span>
                {[1, 2, 3, 4, 5].map((star) => (
                  <button
                    type="button"
                    key={star}
                    onClick={() => setRating(star)}
                    className="p-0.5 text-amber-400"
                  >
                    <Star className={`w-4 h-4 ${star <= rating ? 'fill-current' : 'text-slate-200'}`} />
                  </button>
                ))}
              </div>
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-3 py-1.5 bg-slate-900 text-white rounded-lg text-xs font-medium hover:bg-slate-800 disabled:opacity-50"
              >
                {isSubmitting ? 'Posting...' : 'Post Review'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};
