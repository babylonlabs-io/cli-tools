package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/babylonlabs-io/cli-tools/internal/config"
	"github.com/babylonlabs-io/cli-tools/internal/db/model"
)

type Database struct {
	DbName string
	Client *mongo.Client
}

func New(ctx context.Context, cfg config.DbConfig) (*Database, error) {
	credential := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clientOps := options.Client().ApplyURI(cfg.Address).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return nil, err
	}

	return &Database{
		DbName: cfg.DbName,
		Client: client,
	}, nil
}

func (db *Database) Ping(ctx context.Context) error {
	err := db.Client.Ping(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) SaveUnbondingDocument(
	ctx context.Context,
	unbondingTxHashHex string,
	unbondingTxHex string,
	unbondingTxSigHex string,
	stakerPkHex string,
	finalityPkHex string,
	stakingTxHex string,
	stakingOutputIndex uint64,
	stakingTxHashHex string,
	stakingTime uint64,
	stakingAmount uint64,
) error {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)
	document := model.UnbondingDocument{
		StakerPkHex:        stakerPkHex,
		FinalityPkHex:      finalityPkHex,
		UnbondingTxSigHex:  unbondingTxSigHex,
		State:              model.Inserted,
		UnbondingTxHashHex: unbondingTxHashHex,
		UnbondingTxHex:     unbondingTxHex,
		StakingTxHex:       stakingTxHex,
		StakingOutputIndex: stakingOutputIndex,
		StakingTimelock:    stakingTime,
		StakingAmount:      stakingAmount,
		StakingTxHashHex:   stakingTxHashHex,
	}
	_, err := client.InsertOne(ctx, document)

	return err

}

func (db *Database) findUnbondingDocumentsWithState(ctx context.Context, state model.UnbondingState) ([]model.UnbondingDocument, error) {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)

	filter := bson.M{"state": state}
	options := options.Find().SetSort(bson.M{"_id": 1}) // Sorting in ascending order

	cursor, err := client.Find(ctx, filter, options)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []model.UnbondingDocument
	if err = cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
}

func (db *Database) FindNewUnbondingDocuments(ctx context.Context) ([]model.UnbondingDocument, error) {
	return db.findUnbondingDocumentsWithState(ctx, model.Inserted)
}

func (db *Database) FindFailedUnbodningDocuments(ctx context.Context) ([]model.UnbondingDocument, error) {
	return db.findUnbondingDocumentsWithState(ctx, model.Failed)
}

func (db *Database) FindUnbondingDocumentsWithNoCovenantQuorum(ctx context.Context) ([]model.UnbondingDocument, error) {
	return db.findUnbondingDocumentsWithState(ctx, model.FailedToGetCovenantSignatures)
}

func (db *Database) FindSendUnbondingDocuments(ctx context.Context) ([]model.UnbondingDocument, error) {
	return db.findUnbondingDocumentsWithState(ctx, model.Send)
}

func (db *Database) updateUnbondingDocumentState(
	ctx context.Context,
	id primitive.ObjectID,
	newState model.UnbondingState) error {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"state": newState}}
	_, err := client.UpdateOne(ctx, filter, update)
	return err
}

func (db *Database) SetUnbondingDocumentSend(
	ctx context.Context,
	id primitive.ObjectID) error {
	return db.updateUnbondingDocumentState(ctx, id, model.Send)
}

func (db *Database) SetUnbondingDocumentFailed(
	ctx context.Context,
	id primitive.ObjectID) error {
	return db.updateUnbondingDocumentState(ctx, id, model.Failed)
}

func (db *Database) SetUnbondingDocumentInputAlreadySpent(
	ctx context.Context,
	id primitive.ObjectID) error {
	return db.updateUnbondingDocumentState(ctx, id, model.InputAlreadySpent)
}

func (db *Database) SetUnbondingDocumentFailedToGetCovenantSignatures(
	ctx context.Context,
	id primitive.ObjectID) error {
	return db.updateUnbondingDocumentState(ctx, id, model.FailedToGetCovenantSignatures)
}
