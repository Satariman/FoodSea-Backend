-- Add GIN full-text search index on products using Russian dictionary.
-- This index cannot be expressed through Ent annotations, so it is added manually.
CREATE INDEX idx_products_search ON products
    USING GIN (to_tsvector('russian',
        name || ' ' || coalesce(description, '') || ' ' || coalesce(composition, '')
    ));
