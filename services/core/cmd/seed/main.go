package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	entdialect "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/agext/levenshtein"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/foodsea/core/ent"
	entcategory "github.com/foodsea/core/ent/category"
	entdc "github.com/foodsea/core/ent/deliverycondition"
	_ "github.com/foodsea/core/ent/runtime"
	entoffer "github.com/foodsea/core/ent/offer"
	entproduct "github.com/foodsea/core/ent/product"
	entstore "github.com/foodsea/core/ent/store"
	platforms3 "github.com/foodsea/core/internal/platform/s3"
)

// ---- JSON types ----

type productJSON struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Subcategory string         `json:"subcategory"`
	Brand       string         `json:"brand"`
	Weight      string         `json:"weight"`
	Price       float64        `json:"price"`
	Description string         `json:"description"`
	Composition string         `json:"composition"`
	Nutrition   *nutritionJSON `json:"nutrition"`
	InStock     bool           `json:"in_stock"`
	Promotion   *promotionJSON `json:"promotion"`
}

type nutritionJSON struct {
	Calories      float64 `json:"calories"`
	Protein       float64 `json:"protein"`
	Fat           float64 `json:"fat"`
	Carbohydrates float64 `json:"carbohydrates"`
}

type promotionJSON struct {
	DiscountPercent   int     `json:"discount_percent"`
	PriceWithDiscount float64 `json:"price_with_discount"`
}

type imageIndexEntry struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	File  string `json:"file"`
}

// ---- Internal records ----

type productRecord struct {
	Data      productJSON
	CatFolder string // e.g. "Молочные_продукты_яйца"
	SubFolder string // e.g. "Йогурт"
}

type offerRecord struct {
	ProductID            uuid.UUID
	StoreSlug            string
	PriceKopecks         int
	DiscountPercent      int8
	OriginalPriceKopecks *int
	InStock              bool
}

type offerKey struct {
	ProductID uuid.UUID
	StoreID   uuid.UUID
}

// ---- Stats ----

type stats struct {
	storesCreated    int
	categoriesTotal  int
	brandsCreated    int
	productsCreated  int
	productsSkipped  int
	nutritionCreated int
	offersCreated    int
	offersUpdated    int
	barcodesUpdated  int
	imagesUploaded   int
	imagesSkipped    int
	errors           int
}

// ---- Seeder ----

type seeder struct {
	db           *ent.Client
	s3           *platforms3.Client
	log          *slog.Logger
	productsDir  string
	imagesDir    string
	barcodesFile string
	stats        stats
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	dbURL := getenv("CORE_DB_URL", "postgres://postgres:postgres@localhost:5433/core_db?sslmode=disable")
	productsDir := getenv("PRODUCTS_DIR", "/Users/mihailbasin/PycharmProjects/GenerateProductData/products")
	imagesDir := getenv("IMAGES_DIR", "/Users/mihailbasin/PycharmProjects/SupermarketAPITest/chizhik_images")
	barcodesFile := getenv("BARCODES_FILE", "/Users/mihailbasin/PycharmProjects/GenerateProductData/barcode_progress.json")
	s3Endpoint := getenv("S3_ENDPOINT", "localhost:9000")
	s3AccessKey := getenv("S3_ACCESS_KEY", "minioadmin")
	s3SecretKey := getenv("S3_SECRET_KEY", "minioadmin")
	s3Bucket := getenv("S3_BUCKET", "product-images")
	s3PublicURL := getenv("S3_PUBLIC_URL", "http://localhost:9000/product-images")

	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		log.Error("ping db", "err", err)
		os.Exit(1)
	}
	log.Info("database connected")

	drv := entsql.OpenDB(entdialect.Postgres, sqlDB)
	client := ent.NewClient(ent.Driver(drv))
	defer client.Close()

	s3Client, err := platforms3.NewClient(ctx, platforms3.Config{
		Endpoint:        s3Endpoint,
		AccessKeyID:     s3AccessKey,
		SecretAccessKey: s3SecretKey,
		BucketName:      s3Bucket,
		PublicBaseURL:   s3PublicURL,
	})
	if err != nil {
		log.Error("connect minio", "err", err)
		os.Exit(1)
	}
	log.Info("minio connected")

	s := &seeder{
		db:           client,
		s3:           s3Client,
		log:          log,
		productsDir:  productsDir,
		imagesDir:    imagesDir,
		barcodesFile: barcodesFile,
	}

	if err := s.run(ctx); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}

	log.Info("seed complete",
		"stores_created", s.stats.storesCreated,
		"categories_total", s.stats.categoriesTotal,
		"brands_created", s.stats.brandsCreated,
		"products_created", s.stats.productsCreated,
		"products_skipped", s.stats.productsSkipped,
		"nutrition_created", s.stats.nutritionCreated,
		"offers_created", s.stats.offersCreated,
		"offers_updated", s.stats.offersUpdated,
		"barcodes_updated", s.stats.barcodesUpdated,
		"images_uploaded", s.stats.imagesUploaded,
		"images_skipped", s.stats.imagesSkipped,
		"errors", s.stats.errors,
	)
}

func (s *seeder) run(ctx context.Context) error {
	storeIDs, err := s.seedStores(ctx)
	if err != nil {
		return fmt.Errorf("stores: %w", err)
	}
	s.log.Info("[stores] done", "count", len(storeIDs))

	if err := s.seedDeliveryConditions(ctx, storeIDs); err != nil {
		return fmt.Errorf("delivery conditions: %w", err)
	}
	s.log.Info("[delivery] done", "count", len(storeIDs))

	catIDs, subIDs, err := s.seedCategories(ctx)
	if err != nil {
		return fmt.Errorf("categories: %w", err)
	}
	s.log.Info("[categories] done", "categories", len(catIDs), "subcategories", len(subIDs))

	brandNames, products, offers, err := s.collectData()
	if err != nil {
		return fmt.Errorf("collect data: %w", err)
	}
	s.log.Info("[collect] done", "unique_products", len(products), "offers", len(offers), "brands", len(brandNames))

	brandIDs, err := s.seedBrands(ctx, brandNames)
	if err != nil {
		return fmt.Errorf("brands: %w", err)
	}
	s.log.Info("[brands] done", "created", s.stats.brandsCreated)

	if err := s.seedProducts(ctx, products, catIDs, subIDs, brandIDs); err != nil {
		return fmt.Errorf("products: %w", err)
	}
	s.log.Info("[products] done", "created", s.stats.productsCreated, "skipped", s.stats.productsSkipped)

	if err := s.seedOffers(ctx, offers, storeIDs); err != nil {
		return fmt.Errorf("offers: %w", err)
	}
	s.log.Info("[offers] done", "created", s.stats.offersCreated, "updated", s.stats.offersUpdated)

	if err := s.seedBarcodes(ctx); err != nil {
		return fmt.Errorf("barcodes: %w", err)
	}
	s.log.Info("[barcodes] done", "updated", s.stats.barcodesUpdated)

	if err := s.seedImages(ctx, subIDs); err != nil {
		return fmt.Errorf("images: %w", err)
	}
	s.log.Info("[images] done", "uploaded", s.stats.imagesUploaded, "skipped", s.stats.imagesSkipped)

	return nil
}

// ---- Step 1: Stores ----

func (s *seeder) seedStores(ctx context.Context) (map[string]uuid.UUID, error) {
	definitions := []struct{ name, slug string }{
		{"Чижик", "chizhik"},
		{"Магнит", "magnit"},
		{"Перекрёсток", "perekrestok"},
		{"Пятёрочка", "pyaterochka"},
	}

	result := make(map[string]uuid.UUID, len(definitions))
	for _, def := range definitions {
		st, err := s.db.Store.Query().Where(entstore.SlugEQ(def.slug)).Only(ctx)
		if ent.IsNotFound(err) {
			st, err = s.db.Store.Create().
				SetName(def.name).
				SetSlug(def.slug).
				SetCreatedAt(time.Now()).
				Save(ctx)
			if err != nil {
				return nil, fmt.Errorf("create store %q: %w", def.slug, err)
			}
			s.stats.storesCreated++
		} else if err != nil {
			return nil, fmt.Errorf("query store %q: %w", def.slug, err)
		}
		result[def.slug] = st.ID
	}
	return result, nil
}

func (s *seeder) seedDeliveryConditions(ctx context.Context, storeIDs map[string]uuid.UUID) error {
	type deliveryPolicy struct {
		MinOrderKopecks     int
		DeliveryCostKopecks int
		FreeFromKopecks     *int
		EstimatedMinutes    *int
	}

	i := func(v int) *int { return &v }

	policies := map[string]deliveryPolicy{
		"chizhik": {
			MinOrderKopecks:     0,
			DeliveryCostKopecks: 14900,
			FreeFromKopecks:     i(180000),
			EstimatedMinutes:    i(45),
		},
		"magnit": {
			MinOrderKopecks:     50000,
			DeliveryCostKopecks: 9900,
			FreeFromKopecks:     i(150000),
			EstimatedMinutes:    i(35),
		},
		"perekrestok": {
			MinOrderKopecks:     120000,
			DeliveryCostKopecks: 17900,
			FreeFromKopecks:     i(250000),
			EstimatedMinutes:    i(55),
		},
		"pyaterochka": {
			MinOrderKopecks:     80000,
			DeliveryCostKopecks: 12900,
			FreeFromKopecks:     i(200000),
			EstimatedMinutes:    i(40),
		},
	}

	for slug, storeID := range storeIDs {
		policy, ok := policies[slug]
		if !ok {
			continue
		}

		existing, err := s.db.DeliveryCondition.Query().
			Where(entdc.StoreIDEQ(storeID)).
			Only(ctx)
		if ent.IsNotFound(err) {
			if _, err = s.db.DeliveryCondition.Create().
				SetStoreID(storeID).
				SetMinOrderKopecks(policy.MinOrderKopecks).
				SetDeliveryCostKopecks(policy.DeliveryCostKopecks).
				SetNillableFreeFromKopecks(policy.FreeFromKopecks).
				SetNillableEstimatedMinutes(policy.EstimatedMinutes).
				Save(ctx); err != nil {
				return fmt.Errorf("create delivery condition for %q: %w", slug, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("query delivery condition for %q: %w", slug, err)
		}

		if err := s.db.DeliveryCondition.UpdateOneID(existing.ID).
			SetMinOrderKopecks(policy.MinOrderKopecks).
			SetDeliveryCostKopecks(policy.DeliveryCostKopecks).
			SetNillableFreeFromKopecks(policy.FreeFromKopecks).
			SetNillableEstimatedMinutes(policy.EstimatedMinutes).
			Exec(ctx); err != nil {
			return fmt.Errorf("update delivery condition for %q: %w", slug, err)
		}
	}

	return nil
}

// ---- Step 2: Categories ----

func (s *seeder) seedCategories(ctx context.Context) (catIDs map[string]uuid.UUID, subIDs map[string]uuid.UUID, err error) {
	catIDs = make(map[string]uuid.UUID)
	subIDs = make(map[string]uuid.UUID)

	catFolders, err := os.ReadDir(s.productsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read products dir: %w", err)
	}

	for _, catEntry := range catFolders {
		if !catEntry.IsDir() {
			continue
		}
		catFolder := catEntry.Name()
		catSlug := strings.ToLower(catFolder)
		catName := s.readCategoryName(filepath.Join(s.productsDir, catFolder))

		cat, err := s.upsertCategory(ctx, catName, catSlug, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("upsert category %q: %w", catSlug, err)
		}
		catIDs[catFolder] = cat.ID
		s.stats.categoriesTotal++

		subFolders, err := os.ReadDir(filepath.Join(s.productsDir, catFolder))
		if err != nil {
			return nil, nil, fmt.Errorf("read subdir %q: %w", catFolder, err)
		}

		for _, subEntry := range subFolders {
			if !subEntry.IsDir() {
				continue
			}
			subFolder := subEntry.Name()
			subSlug := catSlug + "_" + strings.ToLower(subFolder)
			subName := strings.ReplaceAll(subFolder, "_", " ")

			sub, err := s.upsertCategory(ctx, subName, subSlug, &cat.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("upsert subcategory %q: %w", subSlug, err)
			}
			subIDs[catFolder+"/"+subFolder] = sub.ID
			s.stats.categoriesTotal++
		}
	}

	return catIDs, subIDs, nil
}

func (s *seeder) upsertCategory(ctx context.Context, name, slug string, parentID *uuid.UUID) (*ent.Category, error) {
	cat, err := s.db.Category.Query().Where(entcategory.SlugEQ(slug)).Only(ctx)
	if ent.IsNotFound(err) {
		c := s.db.Category.Create().
			SetName(name).
			SetSlug(slug).
			SetCreatedAt(time.Now())
		if parentID != nil {
			c = c.SetParentID(*parentID)
		}
		return c.Save(ctx)
	}
	return cat, err
}

func (s *seeder) readCategoryName(catDirPath string) string {
	subDirs, _ := os.ReadDir(catDirPath)
	for _, sub := range subDirs {
		if !sub.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(catDirPath, sub.Name()))
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(catDirPath, sub.Name(), f.Name()))
			if err != nil {
				continue
			}
			var items []productJSON
			if err := json.Unmarshal(data, &items); err != nil || len(items) == 0 {
				continue
			}
			return items[0].Category
		}
	}
	return strings.ReplaceAll(filepath.Base(catDirPath), "_", " ")
}

// ---- Collect data ----

func (s *seeder) collectData() (
	brandNames map[string]struct{},
	products map[uuid.UUID]productRecord,
	offers []offerRecord,
	err error,
) {
	brandNames = make(map[string]struct{})
	products = make(map[uuid.UUID]productRecord)

	catFolders, err := os.ReadDir(s.productsDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read products dir: %w", err)
	}

	for _, catEntry := range catFolders {
		if !catEntry.IsDir() {
			continue
		}
		catFolder := catEntry.Name()

		subFolders, err := os.ReadDir(filepath.Join(s.productsDir, catFolder))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read subdir: %w", err)
		}

		for _, subEntry := range subFolders {
			if !subEntry.IsDir() {
				continue
			}
			subFolder := subEntry.Name()
			subPath := filepath.Join(s.productsDir, catFolder, subFolder)

			jsonFiles, err := os.ReadDir(subPath)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("read json dir: %w", err)
			}

			for _, jf := range jsonFiles {
				if !strings.HasSuffix(jf.Name(), ".json") {
					continue
				}
				// Parse store slug from filename: "Йогурт_chizhik.json" → "chizhik"
				nameParts := strings.SplitN(strings.TrimSuffix(jf.Name(), ".json"), "_", 2)
				if len(nameParts) < 2 {
					continue
				}
				// The store slug is the last _-separated part of the filename (without extension).
				// Filename format: "{Subcategory}_{storeSlug}.json"
				// But subcategory can have underscores too, so take the LAST part.
				parts := strings.Split(strings.TrimSuffix(jf.Name(), ".json"), "_")
				storeSlug := parts[len(parts)-1]

				data, err := os.ReadFile(filepath.Join(subPath, jf.Name()))
				if err != nil {
					s.log.Warn("read json", "file", jf.Name(), "err", err)
					continue
				}

				var items []productJSON
				if err := json.Unmarshal(data, &items); err != nil {
					s.log.Warn("parse json", "file", jf.Name(), "err", err)
					continue
				}

				for _, item := range items {
					id, err := uuid.Parse(item.ID)
					if err != nil {
						s.log.Warn("bad uuid", "id", item.ID)
						continue
					}

					if item.Brand != "" {
						brandNames[item.Brand] = struct{}{}
					}

					if _, exists := products[id]; !exists {
						products[id] = productRecord{
							Data:      item,
							CatFolder: catFolder,
							SubFolder: subFolder,
						}
					}

					priceKopecks := int(item.Price * 100)
					var discountPercent int8
					var originalPriceKopecks *int

					if item.Promotion != nil {
						discountPercent = int8(item.Promotion.DiscountPercent)
						orig := priceKopecks
						originalPriceKopecks = &orig
						priceKopecks = int(item.Promotion.PriceWithDiscount * 100)
					}

					offers = append(offers, offerRecord{
						ProductID:            id,
						StoreSlug:            storeSlug,
						PriceKopecks:         priceKopecks,
						DiscountPercent:      discountPercent,
						OriginalPriceKopecks: originalPriceKopecks,
						InStock:              item.InStock,
					})
				}
			}
		}
	}

	return brandNames, products, offers, nil
}

// ---- Step 3: Brands ----

func (s *seeder) seedBrands(ctx context.Context, names map[string]struct{}) (map[string]uuid.UUID, error) {
	existing, err := s.db.Brand.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load brands: %w", err)
	}

	result := make(map[string]uuid.UUID, len(existing))
	for _, b := range existing {
		result[b.Name] = b.ID
	}

	for name := range names {
		if _, ok := result[name]; ok {
			continue
		}
		b, err := s.db.Brand.Create().
			SetName(name).
			SetCreatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			s.log.Error("create brand", "name", name, "err", err)
			s.stats.errors++
			continue
		}
		result[name] = b.ID
		s.stats.brandsCreated++
	}

	return result, nil
}

// ---- Step 4: Products + Nutrition ----

func (s *seeder) seedProducts(
	ctx context.Context,
	products map[uuid.UUID]productRecord,
	catIDs map[string]uuid.UUID,
	subIDs map[string]uuid.UUID,
	brandIDs map[string]uuid.UUID,
) error {
	existingIDs, err := s.db.Product.Query().IDs(ctx)
	if err != nil {
		return fmt.Errorf("load existing products: %w", err)
	}

	existingSet := make(map[uuid.UUID]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = struct{}{}
	}

	type pendingProduct struct {
		id  uuid.UUID
		rec productRecord
	}

	var pending []pendingProduct
	for id, rec := range products {
		if _, exists := existingSet[id]; exists {
			s.stats.productsSkipped++
			continue
		}
		pending = append(pending, pendingProduct{id, rec})
	}

	const batchSize = 100
	for i := 0; i < len(pending); i += batchSize {
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]

		creates := make([]*ent.ProductCreate, 0, len(batch))
		for _, pp := range batch {
			catID, ok := catIDs[pp.rec.CatFolder]
			if !ok {
				s.log.Warn("category not found", "folder", pp.rec.CatFolder)
				s.stats.errors++
				continue
			}

			c := s.db.Product.Create().
				SetID(pp.id).
				SetName(pp.rec.Data.Name).
				SetCategoryID(catID).
				SetInStock(pp.rec.Data.InStock).
				SetNillableDescription(nilIfEmpty(pp.rec.Data.Description)).
				SetNillableComposition(nilIfEmpty(pp.rec.Data.Composition)).
				SetNillableWeight(nilIfEmpty(pp.rec.Data.Weight))

			subKey := pp.rec.CatFolder + "/" + pp.rec.SubFolder
			if subID, ok := subIDs[subKey]; ok {
				c = c.SetSubcategoryID(subID)
			}

			if pp.rec.Data.Brand != "" {
				if brandID, ok := brandIDs[pp.rec.Data.Brand]; ok {
					c = c.SetBrandID(brandID)
				}
			}

			creates = append(creates, c)
		}

		if len(creates) == 0 {
			continue
		}

		if err := s.db.Product.CreateBulk(creates...).Exec(ctx); err != nil {
			s.log.Error("bulk create products", "batch_start", i, "err", err)
			s.stats.errors++
			continue
		}
		s.stats.productsCreated += len(creates)

		// Create nutrition for this batch
		var nutCreates []*ent.ProductNutritionCreate
		for _, pp := range batch {
			if pp.rec.Data.Nutrition == nil {
				continue
			}
			nutCreates = append(nutCreates, s.db.ProductNutrition.Create().
				SetProductID(pp.id).
				SetCalories(pp.rec.Data.Nutrition.Calories).
				SetProtein(pp.rec.Data.Nutrition.Protein).
				SetFat(pp.rec.Data.Nutrition.Fat).
				SetCarbohydrates(pp.rec.Data.Nutrition.Carbohydrates))
		}
		if len(nutCreates) > 0 {
			if err := s.db.ProductNutrition.CreateBulk(nutCreates...).Exec(ctx); err != nil {
				s.log.Error("bulk create nutrition", "batch_start", i, "err", err)
				s.stats.errors++
			} else {
				s.stats.nutritionCreated += len(nutCreates)
			}
		}

		if i%500 == 0 || end == len(pending) {
			s.log.Info("[products] progress", "created", s.stats.productsCreated, "total_new", len(pending))
		}
	}

	return nil
}

// ---- Step 5: Offers ----

func (s *seeder) seedOffers(ctx context.Context, offers []offerRecord, storeIDs map[string]uuid.UUID) error {
	existing, err := s.db.Offer.Query().
		Select(entoffer.FieldID, entoffer.FieldProductID, entoffer.FieldStoreID).
		All(ctx)
	if err != nil {
		return fmt.Errorf("load existing offers: %w", err)
	}

	existingMap := make(map[offerKey]uuid.UUID, len(existing))
	for _, o := range existing {
		existingMap[offerKey{o.ProductID, o.StoreID}] = o.ID
	}

	type updateTask struct {
		id  uuid.UUID
		rec offerRecord
	}

	var toCreate []*ent.OfferCreate
	var toUpdate []updateTask

	for _, r := range offers {
		storeID, ok := storeIDs[r.StoreSlug]
		if !ok {
			s.log.Warn("unknown store slug", "slug", r.StoreSlug)
			continue
		}
		key := offerKey{r.ProductID, storeID}
		if existingID, found := existingMap[key]; found {
			toUpdate = append(toUpdate, updateTask{existingID, r})
		} else {
			toCreate = append(toCreate, s.db.Offer.Create().
				SetProductID(r.ProductID).
				SetStoreID(storeID).
				SetPriceKopecks(r.PriceKopecks).
				SetDiscountPercent(r.DiscountPercent).
				SetNillableOriginalPriceKopecks(r.OriginalPriceKopecks).
				SetInStock(r.InStock))
		}
	}

	const batchSize = 100
	for i := 0; i < len(toCreate); i += batchSize {
		end := i + batchSize
		if end > len(toCreate) {
			end = len(toCreate)
		}
		if err := s.db.Offer.CreateBulk(toCreate[i:end]...).Exec(ctx); err != nil {
			s.log.Error("bulk create offers", "batch_start", i, "err", err)
			s.stats.errors++
		} else {
			s.stats.offersCreated += end - i
		}
	}

	for _, task := range toUpdate {
		upd := s.db.Offer.UpdateOneID(task.id).
			SetPriceKopecks(task.rec.PriceKopecks).
			SetDiscountPercent(task.rec.DiscountPercent).
			SetInStock(task.rec.InStock).
			SetNillableOriginalPriceKopecks(task.rec.OriginalPriceKopecks)
		if task.rec.OriginalPriceKopecks == nil {
			upd = upd.ClearOriginalPriceKopecks()
		}
		if err := upd.Exec(ctx); err != nil {
			s.log.Error("update offer", "id", task.id, "err", err)
			s.stats.errors++
		} else {
			s.stats.offersUpdated++
		}
	}

	return nil
}

// ---- Step 6: Barcodes ----

func (s *seeder) seedBarcodes(ctx context.Context) error {
	data, err := os.ReadFile(s.barcodesFile)
	if err != nil {
		return fmt.Errorf("read barcodes file: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse barcodes json: %w", err)
	}

	for idStr, val := range raw {
		if val == nil {
			continue
		}
		barcode, ok := val.(string)
		if !ok || barcode == "" {
			continue
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}

		if err := s.db.Product.UpdateOneID(id).SetBarcode(barcode).Exec(ctx); err != nil {
			if ent.IsNotFound(err) {
				continue
			}
			s.log.Warn("set barcode", "product_id", id, "err", err)
			s.stats.errors++
			continue
		}
		s.stats.barcodesUpdated++
	}

	return nil
}

// ---- Step 7: Images ----

func (s *seeder) seedImages(ctx context.Context, subIDs map[string]uuid.UUID) error {
	return filepath.WalkDir(s.imagesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "index.json" {
			return err
		}

		// Derive catFolder / subFolder from path
		rel, err := filepath.Rel(s.imagesDir, filepath.Dir(path))
		if err != nil {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 2 {
			return nil
		}
		catFolder, subFolder := parts[0], parts[1]
		subKey := catFolder + "/" + subFolder

		subID, ok := subIDs[subKey]
		if !ok {
			s.log.Warn("[images] subcategory not found", "key", subKey)
			return nil
		}

		return s.processImageIndex(ctx, path, subID)
	})
}

func (s *seeder) processImageIndex(ctx context.Context, indexPath string, subID uuid.UUID) error {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}

	var entries []imageIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	// Load all products in this subcategory that don't have an image yet.
	prods, err := s.db.Product.Query().
		Where(
			entproduct.SubcategoryID(subID),
			entproduct.ImageURLIsNil(),
		).
		Select(entproduct.FieldID, entproduct.FieldName).
		All(ctx)
	if err != nil {
		s.log.Error("[images] load products", "subID", subID, "err", err)
		return nil
	}

	if len(prods) == 0 {
		return nil
	}

	dir := filepath.Dir(indexPath)

	for _, entry := range entries {
		imgPath := filepath.Join(dir, entry.File)
		if _, err := os.Stat(imgPath); err != nil {
			s.stats.imagesSkipped++
			continue
		}

		matched := bestMatch(entry.Title, prods)
		if matched == nil {
			s.log.Debug("[images] no match", "title", entry.Title)
			s.stats.imagesSkipped++
			continue
		}

		imgURL, err := s.uploadImage(ctx, matched.ID, entry.File, imgPath)
		if err != nil {
			s.log.Error("[images] upload", "file", entry.File, "err", err)
			s.stats.errors++
			continue
		}

		if err := s.db.Product.UpdateOneID(matched.ID).SetImageURL(imgURL).Exec(ctx); err != nil {
			s.log.Error("[images] update product", "id", matched.ID, "err", err)
			s.stats.errors++
			continue
		}

		// Remove matched product from slice to avoid duplicate assignment.
		prods = removeProduct(prods, matched.ID)
		s.stats.imagesUploaded++
	}

	return nil
}

func (s *seeder) uploadImage(ctx context.Context, productID uuid.UUID, filename, localPath string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(filename))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "image/jpeg"
	}

	key := fmt.Sprintf("products/%s/%s", productID, filename)
	return s.s3.Upload(ctx, key, f, contentType)
}

// ---- Helpers ----

func bestMatch(title string, prods []*ent.Product) *ent.Product {
	normTitle := normalize(title)
	var best *ent.Product
	var bestSim float64

	for _, p := range prods {
		sim := levenshtein.Similarity(normTitle, normalize(p.Name), nil)
		if sim > bestSim {
			bestSim = sim
			best = p
		}
	}

	// No hard threshold: images are pre-filtered by subcategory, so any
	// image from the right category is better than none.
	return best
}

func removeProduct(prods []*ent.Product, id uuid.UUID) []*ent.Product {
	for i, p := range prods {
		if p.ID == id {
			return append(prods[:i], prods[i+1:]...)
		}
	}
	return prods
}

func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
